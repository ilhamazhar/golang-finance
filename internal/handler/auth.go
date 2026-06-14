package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ilhamazhar/golang-gpt/internal/domain"
	"github.com/ilhamazhar/golang-gpt/internal/middleware"
	"github.com/ilhamazhar/golang-gpt/pkg/response"
)

type AuthHandler struct {
	auth domain.AuthService
}

func NewAuthHandler(auth domain.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req domain.RegisterRequest
	if !bindJSON(c, &req) {
		return
	}

	user, err := h.auth.Register(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, http.StatusConflict, err.Error(), nil)
		return
	}

	response.OK(c, http.StatusCreated, "Registered successfully", user)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if !bindJSON(c, &req) {
		return
	}

	tokens, err := h.auth.Login(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, err.Error(), nil)
		return
	}

	response.OK(c, http.StatusOK, "Logged in successfully", tokens)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req domain.RefreshRequest
	if !bindJSON(c, &req) {
		return
	}

	tokens, err := h.auth.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, err.Error(), nil)
		return
	}

	response.OK(c, http.StatusOK, "Token refreshed", tokens)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req domain.LogoutRequest
	if !bindJSON(c, &req) {
		return
	}

	if err := h.auth.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		response.Fail(c, http.StatusUnauthorized, err.Error(), nil)
		return
	}

	response.OK(c, http.StatusOK, "Logged out successfully", nil)
}

func (h *AuthHandler) Me(c *gin.Context) {
	claims := middleware.ClaimsFromContext(c)

	user, err := h.auth.GetProfile(c.Request.Context(), claims.UserID)
	if err != nil {
		response.Fail(c, http.StatusNotFound, err.Error(), nil)
		return
	}

	response.OK(c, http.StatusOK, "User info retrieved", user)
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	claims := middleware.ClaimsFromContext(c)

	var req domain.ChangePasswordRequest
	if !bindJSON(c, &req) {
		return
	}

	if err := h.auth.ChangePassword(c.Request.Context(), claims.UserID, req); err != nil {
		response.Fail(c, http.StatusUnprocessableEntity, err.Error(), nil)
		return
	}

	response.OK(c, http.StatusOK, "Password changed successfully", nil)
}
