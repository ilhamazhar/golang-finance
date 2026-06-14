package handler

import (
	"errors"
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

	result, err := h.auth.Register(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, http.StatusConflict, err.Error(), nil)
		return
	}

	response.OK(c, http.StatusCreated, "Registered successfully. Please verify your email.", result)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if !bindJSON(c, &req) {
		return
	}

	tokens, err := h.auth.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, domain.ErrEmailNotVerified) {
			response.Fail(c, http.StatusForbidden, err.Error(), nil)
			return
		}
		response.Fail(c, http.StatusUnauthorized, err.Error(), nil)
		return
	}

	response.OK(c, http.StatusOK, "Logged in successfully", tokens)
}

func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")

	if err := h.auth.VerifyEmail(c.Request.Context(), token); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	response.OK(c, http.StatusOK, "Email verified successfully", nil)
}

func (h *AuthHandler) ResendVerification(c *gin.Context) {
	var req domain.ResendVerificationRequest
	if !bindJSON(c, &req) {
		return
	}

	token, err := h.auth.ResendVerification(c.Request.Context(), req.Email)
	if err != nil {
		response.Fail(c, http.StatusConflict, err.Error(), nil)
		return
	}

	var data any
	if token != "" {
		data = gin.H{"verification_token": token}
	}
	response.OK(c, http.StatusOK, "If the email exists and is unverified, a verification link has been sent", data)
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
