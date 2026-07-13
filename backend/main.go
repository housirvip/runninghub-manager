package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "runninghub-manager/apps"
	"runninghub-manager/config"
	"runninghub-manager/database"
	"runninghub-manager/handlers"
	"runninghub-manager/middleware"
	"runninghub-manager/models"
	"runninghub-manager/services"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Load config
	cfg := config.Load()

	// Init database
	db := database.Init(cfg)
	// Load persisted settings from DB (overrides env defaults; env remains the fallback)
	if err := config.AppConfig.LoadFromDB(db); err != nil {
		log.Printf("warn: load settings from db: %v", err)
	}

	// Create default admin if no users exist
	var userCount int64
	db.Model(&models.User{}).Count(&userCount)
	if userCount == 0 {
		adminPass := os.Getenv("ADMIN_PASSWORD")
		if adminPass == "" {
			// Generate random password and print it
			b := make([]byte, 12)
			rand.Read(b)
			adminPass = hex.EncodeToString(b)[:16]
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("Failed to hash admin password: %v", err)
		}
		admin := models.User{
			Username:     "admin",
			PasswordHash: string(hash),
			IsAdmin:      true,
		}
		db.Create(&admin)
		log.Printf("⚠️  Default admin created (username: admin, password: %s). Change it immediately!", adminPass)
	}

	// Init RunningHub client
	rhClient := services.NewRHClient(cfg.RHBaseURL)

	// Init scheduler
	scheduler := services.NewScheduler(db, rhClient, cfg.SchedulerTick)
	scheduler.Start()

	// Ensure output and upload directories exist
	os.MkdirAll(cfg.OutputDir, 0755)
	os.MkdirAll(cfg.UploadDir, 0755)

	// Init local app executor
	localExecutor := services.NewLocalExecutor(db, cfg.BaseURL, cfg.UploadDir, cfg.OutputDir, cfg.LocalMaxConc)
	localExecutor.Start()

	// Setup Gin router
	r := gin.Default()

	// CORS — allow all origins for LAN access
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// Request logging for analytics
	r.Use(middleware.RequestLogger(db))

	// --- Public routes ---
	authHandler := handlers.NewAuthHandler(db)
	authGroup := r.Group("/api/auth")
	{
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/register", authHandler.Register)
	}

	// --- Protected management API (blocks platform keys) ---
	apiKeyHandler := handlers.NewApiKeyHandler(db, scheduler)
	taskHandler := handlers.NewTaskHandler(db, rhClient)
	dashboardHandler := handlers.NewDashboardHandler(db, scheduler)
	platformKeyHandler := handlers.NewPlatformKeyHandler(db)

	api := r.Group("/api", middleware.JWTAuth(db), middleware.BlockPlatformKey())
	{
		api.GET("/apikeys", apiKeyHandler.List)
		api.POST("/apikeys", apiKeyHandler.Create)
		api.PUT("/apikeys/:id", apiKeyHandler.Update)
		api.DELETE("/apikeys/:id", apiKeyHandler.Delete)

		api.GET("/tasks", taskHandler.List)
		api.GET("/tasks/:id", taskHandler.Get)
		api.POST("/tasks/:id/cancel", taskHandler.Cancel)

		api.GET("/dashboard/stats", dashboardHandler.GetStats)
		api.GET("/dashboard/charts", dashboardHandler.GetChartData)
		api.GET("/dashboard/logs", dashboardHandler.GetRequestLogs)
		api.GET("/settings/strategy", dashboardHandler.GetStrategy)
		api.PUT("/settings/strategy", dashboardHandler.SetStrategy)
		api.GET("/settings/tick", dashboardHandler.GetTick)
		api.PUT("/settings/tick", dashboardHandler.SetTick)
		api.GET("/settings/poll", dashboardHandler.GetPollConfig)
		api.PUT("/settings/poll", dashboardHandler.SetPollConfig)

		api.GET("/apps", dashboardHandler.GetApps)

		api.GET("/platform-keys", platformKeyHandler.List)
		api.POST("/platform-keys", platformKeyHandler.Create)
		api.DELETE("/platform-keys/:id", platformKeyHandler.Delete)
		api.GET("/platform-keys/:id/reveal", platformKeyHandler.Reveal)
	}

	// --- RunningHub-compatible proxy (JWT + platform key both allowed) ---
	proxyHandler := handlers.NewProxyHandler(db, rhClient, scheduler)
	proxy := r.Group("", middleware.JWTAuth(db))
	{
		proxy.POST("/task/openapi/ai-app/run", proxyHandler.CreateTask)
		proxy.POST("/task/openapi/cancel", proxyHandler.CancelTask)
		proxy.POST("/openapi/v2/query", proxyHandler.QueryTask)
		proxy.POST("/task/openapi/outputs", proxyHandler.QueryTaskOutputs)
		proxy.POST("/task/openapi/status", proxyHandler.QueryTaskStatus)
		proxy.POST("/task/openapi/upload", proxyHandler.Upload)
		proxy.GET("/api/webapp/apiCallDemo", proxyHandler.GetWebappInfo)
	}

	// Serve output files (no auth — direct access for generated files)
	r.Static("/files", cfg.OutputDir)

	// Serve uploaded files (auth required)
	uploadsGroup := r.Group("/uploads", middleware.JWTAuth(db))
	uploadsGroup.Static("/", cfg.UploadDir)

	// Serve frontend static files (production)
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path

		// Never serve SPA fallback for API/proxy paths
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/task/") || strings.HasPrefix(p, "/openapi/") {
			c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": "not found"})
			return
		}

		// Try to serve static file
		filePath := "./static" + p
		if _, err := os.Stat(filePath); err == nil {
			http.ServeFile(c.Writer, c.Request, filePath)
			return
		}
		// SPA fallback: serve index.html for frontend routes
		if _, err := os.Stat("./static/index.html"); err == nil {
			http.ServeFile(c.Writer, c.Request, "./static/index.html")
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": "not found"})
	})

	// Start HTTP server
	srv := &http.Server{
		Addr:    cfg.Port,
		Handler: r,
	}

	go func() {
		log.Printf("Server starting on %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop accepting new requests
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Stop scheduler and workers (waits for in-flight tasks)
	scheduler.Stop()
	localExecutor.Stop()

	log.Println("Server stopped")
}
