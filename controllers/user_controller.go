package controllers

import (
	"errors"
	"net/http"
	"pdm-backend/models"
	"pdm-backend/repositories"
	"pdm-backend/services"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserHandler struct {
	UserRepo    *repositories.UserRepository
	FinanceRepo *repositories.FinanceRepository
}

func NewUserHandler(userRepo *repositories.UserRepository, financeRepo *repositories.FinanceRepository) *UserHandler {
	return &UserHandler{
		UserRepo:    userRepo,
		FinanceRepo: financeRepo,
	}
}

type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

func (h *UserHandler) Register(c *gin.Context) {

	var registerRequest RegisterRequest

	if err := c.ShouldBindJSON(&registerRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "The request format is invalid"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(registerRequest.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while hashing the password"})
		return
	}

	newUser := models.User{
		Name:         registerRequest.Name,
		Email:        registerRequest.Email,
		PasswordHash: string(hashedPassword),
	}

	err = h.UserRepo.CreateUserAndFinance(&newUser)

	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": "That email address is already registered"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Error creating the user and finance"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "message": "User registered successfully"})
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *UserHandler) Login(c *gin.Context) {

	var userRequest LoginRequest

	if err := c.ShouldBindJSON(&userRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "The request format is invalid"})
		return
	}

	user, err := h.UserRepo.GetUserByEmail(userRequest.Email)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid email or password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Server error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(userRequest.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid email or password"})
		return
	}

	identifiers, err := h.UserRepo.GetFinanceAndSavingSubcategoryByUserId(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while resolving the finance id"})
		return
	}

	token, err := services.GenerateJWT(user.ID, user.Name, user.Email, identifiers.FinanceID, identifiers.SavingsID, user.TokenVersion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Could not issue the token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"token":   token,
		"user":    gin.H{"finance_id": identifiers.FinanceID, "id": user.ID, "name": user.Name, "email": user.Email},
	})
}

type UpdateProfileRequest struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required,email"`
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	var updateRequest UpdateProfileRequest

	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "The request format is invalid"})
		return
	}

	userClaims, httpCode, jsonResponse := services.GetClaims(c)
	if userClaims == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	user, err := h.UserRepo.GetUserById(userClaims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "The user does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Server error"})
		return
	}

	user.Name = updateRequest.Name
	user.Email = updateRequest.Email

	if err := h.UserRepo.UpdateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Error updating the profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "The profile was updated successfully",
		"user":    gin.H{"finance_id": userClaims.FinanceID, "id": user.ID, "name": user.Name, "email": user.Email},
	})
}

type UpdatePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

func (h *UserHandler) UpdatePassword(c *gin.Context) {
	var passwordRequest UpdatePasswordRequest

	if err := c.ShouldBindJSON(&passwordRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "The request format is invalid"})
		return
	}

	userClaims, httpCode, jsonResponse := services.GetClaims(c)
	if userClaims == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	user, err := h.UserRepo.GetUserById(userClaims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "The user does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Server error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(passwordRequest.CurrentPassword)); err != nil {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "That does not match your current password"})
		return
	}

	if passwordRequest.NewPassword != passwordRequest.ConfirmPassword {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "The confirmation does not match your new password"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(passwordRequest.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while hashing the password"})
		return
	}

	user.PasswordHash = string(hashedPassword)
	// Invalidates every token issued before this change, including the one
	// used to make this request.
	user.TokenVersion++

	if err := h.UserRepo.UpdateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Error updating the password"})
		return
	}

	// The token this request was authenticated with just became invalid, so
	// hand back one that matches the new version rather than logging the
	// caller out of their own successful request.
	token, err := services.GenerateJWT(user.ID, user.Name, user.Email, userClaims.FinanceID, userClaims.SavingsID, user.TokenVersion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Could not issue a new token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "The password was updated successfully", "token": token})
}
