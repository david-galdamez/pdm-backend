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

type Handler struct {
	UserRepo    *repositories.UserRepository
	FinanceRepo *repositories.FinanzaRepository
}

func NewUserHandler(userRepo *repositories.UserRepository, financeRepo *repositories.FinanzaRepository) *Handler {
	return &Handler{
		UserRepo:    userRepo,
		FinanceRepo: financeRepo,
	}
}

func (h *Handler) Register(c *gin.Context) {

	var newUser models.User

	if err := c.ShouldBindJSON(&newUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "El formato de la petición esta incorrecto"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newUser.Contrasena), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Ocurrio un error al hashaer la contraseña"})
		return
	}

	newUser.Contrasena = string(hashedPassword)

	err = h.UserRepo.CreateUserAndFinance(&newUser)

	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": "El correo electronico ya esta registrado"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Error al crear el usuario y la finanza"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "message": "Usuario registrado con exito"})
}

type LoginRequest struct {
	Email    string `json:"correo"`
	Password string `json:"contrasena"`
}

func (h *Handler) Login(c *gin.Context) {

	var userRequest LoginRequest

	if err := c.ShouldBindJSON(&userRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "El formato de la peticion esta incorrecto"})
		return
	}

	user, err := h.UserRepo.GetUserByEmail(userRequest.Email)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "No hay cuenta asociada a este correo electronico"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"sucess": false, "message": "Error en el servidor"})
		return
	}

	password := user.Contrasena

	if err := bcrypt.CompareHashAndPassword([]byte(password), []byte(userRequest.Password)); err != nil {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "La contraseña proporcionada es incorrecta"})
		return
	}

	identifiers, err := h.UserRepo.GetFinanceAndSavingSubCategoryByUserId(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Ocurrio un error al conseguir el id de la Finanza"})
		return
	}

	token, err := services.GenerateJWT(user.ID, user.Nombre, user.Correo, identifiers.FinanzaId, identifiers.AhorroId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "No se pudo cargar el token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "token": token, "datos_user": gin.H{"finanza_id": identifiers.FinanzaId, "id": user.ID, "nombre": user.Nombre, "correo": user.Correo}})
}

type UpdateProfileRequest struct {
	Name  string `json:"nombre"`
	Email string `json:"correo"`
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	var updateRequest UpdateProfileRequest

	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "El formato de la petición es incorrecto"})
		return
	}

	userClaims, httpCode, jsonResponse := services.GetClaims(c)
	if userClaims == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	user, err := h.UserRepo.GetUserById(userClaims.UserId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "El usuario no existe"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"sucess": false, "message": "Error en el servidor"})
		return
	}

	user.Nombre = updateRequest.Name
	user.Correo = updateRequest.Email

	if err := h.UserRepo.UpdateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Error al modificar datos"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "El perfil ha sido actualizado correctamente", "datos_user": gin.H{"finanza_id": userClaims.FinanzaId, "id": user.ID, "nombre": user.Nombre, "correo": user.Correo}})
}

type UpdatePasswordRequest struct {
	ActualPassword  string `json:"contrasena"`
	NewPassword     string `json:"nueva_contrasena"`
	ConfirmPassword string `json:"confirmar_contrasena"`
}

func (h *Handler) UpdatePassword(c *gin.Context) {
	var passwordRequest UpdatePasswordRequest

	if err := c.ShouldBindJSON(&passwordRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "El formato de la petición es incorrecto"})
		return
	}

	userClaims, httpCode, jsonResponse := services.GetClaims(c)
	if userClaims == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	user, err := h.UserRepo.GetUserById(userClaims.UserId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "El usuario no existe"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"sucess": false, "message": "Error en el servidor"})
		return
	}

	password := user.Contrasena

	if err := bcrypt.CompareHashAndPassword([]byte(password), []byte(passwordRequest.ActualPassword)); err != nil {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "La contraseña no coincide con la actual"})
		return
	}

	if passwordRequest.NewPassword != passwordRequest.ConfirmPassword {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "La confirmación de tu contraseña no coincide con la nueva"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(passwordRequest.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Hubo un error al hashear la contraseña"})
		return
	}

	user.Contrasena = string(hashedPassword)

	if err := h.UserRepo.UpdateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Error al modificar la contraseña"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "La contraseña ha sido actualizado correctamente"})
}
