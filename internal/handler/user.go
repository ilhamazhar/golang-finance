package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ilhamazhar/golang-gpt/internal/domain"
	"github.com/ilhamazhar/golang-gpt/internal/middleware"
	"github.com/ilhamazhar/golang-gpt/pkg/response"
)

type UserHandler struct {
	svc domain.UserService
}

func NewUserHandler(svc domain.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) GetAll(c *gin.Context) {
	page, limit := parsePagination(c)
	search := strings.TrimSpace(c.Query("search"))
	sort := c.Query("sort")
	order := c.Query("order")

	result, total, err := h.svc.FindAll(c.Request.Context(), page, limit, search, sort, order)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	pagination := response.NewPagination(page, limit, total)
	response.OKPaginated(c, http.StatusOK, "Users retrieved", result, pagination)
}

func (h *UserHandler) GetByID(c *gin.Context) {
	id, err := parseUUID(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id", nil)
		return
	}

	result, err := h.svc.FindByID(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, http.StatusNotFound, err.Error(), nil)
		return
	}

	response.OK(c, http.StatusOK, "User retrieved", result)
}

func (h *UserHandler) Update(c *gin.Context) {
	id, err := parseUUID(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id", nil)
		return
	}

	var req domain.UpdateUserRequest
	if !bindJSON(c, &req) {
		return
	}

	result, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, http.StatusConflict, err.Error(), nil)
		return
	}

	response.OK(c, http.StatusOK, "User updated", result)
}

// UpdateRole changes a user's role. Admin-only (gated on users:update via
// Casbin). An admin cannot change their own role, to avoid accidental
// self-lockout from the admin tier.
func (h *UserHandler) UpdateRole(c *gin.Context) {
	id, err := parseUUID(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id", nil)
		return
	}

	if claims := middleware.ClaimsFromContext(c); claims != nil && claims.UserID == id {
		response.Fail(c, http.StatusBadRequest, "you cannot change your own role", nil)
		return
	}

	var req domain.UpdateRoleRequest
	if !bindJSON(c, &req) {
		return
	}

	result, err := h.svc.UpdateRole(c.Request.Context(), id, req.Role)
	if err != nil {
		response.Fail(c, http.StatusNotFound, err.Error(), nil)
		return
	}

	response.OK(c, http.StatusOK, "User role updated", result)
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, err := parseUUID(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id", nil)
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, http.StatusNotFound, err.Error(), nil)
		return
	}

	response.OK(c, http.StatusOK, "User deleted", nil)
}
