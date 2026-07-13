package handlers

import (
	"net/http"
	"time"

	"runninghub-manager/config"
	"runninghub-manager/models"
	"runninghub-manager/pkg"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB *gorm.DB
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=6"`
}

func NewAuthHandler(db *gorm.DB) *AuthHandler {
	return &AuthHandler{DB: db}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	var user models.User
	if err := h.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		pkg.Error(c, http.StatusUnauthorized, "invalid username or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		pkg.Error(c, http.StatusUnauthorized, "invalid username or password")
		return
	}

	token, err := generateToken(user)
	if err != nil {
		pkg.Error(c, http.StatusInternalServerError, "failed to generate token")
		return
	}

	pkg.Success(c, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"isAdmin":  user.IsAdmin,
		},
	})
}

func (h *AuthHandler) Register(c *gin.Context) {
	if !config.AppConfig.AllowRegister {
		// Check if requester is admin
		isAdmin, exists := c.Get("isAdmin")
		if !exists || !isAdmin.(bool) {
			pkg.Error(c, http.StatusForbidden, "registration is disabled")
			return
		}
	}

	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	// Check if username exists
	var count int64
	h.DB.Model(&models.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		pkg.Error(c, http.StatusConflict, "username already exists")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		pkg.Error(c, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user := models.User{
		Username:     req.Username,
		PasswordHash: string(hash),
		IsAdmin:      false,
	}

	if err := h.DB.Create(&user).Error; err != nil {
		pkg.Error(c, http.StatusInternalServerError, "failed to create user")
		return
	}

	pkg.Success(c, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"isAdmin":  user.IsAdmin,
	})
}

func generateToken(user models.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":      float64(user.ID),
		"username": user.Username,
		"isAdmin":  user.IsAdmin,
		"exp":      time.Now().Add(7 * 24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AppConfig.JWTSecret))
}
