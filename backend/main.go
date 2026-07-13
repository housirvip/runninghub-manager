package main

import (
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

	// Create default admin if no users exist
	var userCount int64
	db.Model(&models.User{}).Count(&userCount)
	if userCount == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		admin := models.User{
			Username:     "admin",
			PasswordHash: string(hash),
			IsAdmin:      true,
		}
		db.Create(&admin)
		log.Println("⚠️  Default admin user created (username: admin, password: admin123). Please change the password!")
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

	// --- Public routes ---
	authHandler := handlers.NewAuthHandler(db)
	authGroup := r.Group("/api/auth")
	{
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/register", authHandler.Register)
	}

	// --- Protected management API ---
	apiKeyHandler := handlers.NewApiKeyHandler(db, scheduler)
	taskHandler := handlers.NewTaskHandler(db, rhClient)
	dashboardHandler := handlers.NewDashboardHandler(db, scheduler)
	platformKeyHandler := handlers.NewPlatformKeyHandler(db)

	api := r.Group("/api", middleware.JWTAuth(db))
	{
		api.GET("/apikeys", apiKeyHandler.List)
		api.POST("/apikeys", apiKeyHandler.Create)
		api.PUT("/apikeys/:id", apiKeyHandler.Update)
		api.DELETE("/apikeys/:id", apiKeyHandler.Delete)

		api.GET("/tasks", taskHandler.List)
		api.GET("/tasks/:id", taskHandler.Get)
		api.POST("/tasks/:id/cancel", taskHandler.Cancel)

		api.GET("/dashboard/stats", dashboardHandler.GetStats)
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

	// --- RunningHub-compatible proxy (JWT protected) ---
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

	// Serve uploaded files (no auth)
	r.Static("/uploads", cfg.UploadDir)

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

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		localExecutor.Stop()
		scheduler.Stop()
		os.Exit(0)
	}()

	log.Printf("Server starting on %s", cfg.Port)
	if err := r.Run(cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
