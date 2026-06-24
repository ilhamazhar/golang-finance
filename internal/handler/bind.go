package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ilhamazhar/golang-gpt/internal/domain"
	"github.com/ilhamazhar/golang-gpt/pkg/jwt"
	"github.com/ilhamazhar/golang-gpt/pkg/response"
	"github.com/ilhamazhar/golang-gpt/pkg/validator"
)

// bindJSON binds the request body and validates it.
// Returns false (after writing the error response) if either step fails.
func bindJSON[T any](c *gin.Context, req *T) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid JSON", nil)
		return false
	}
	if errs := validator.Validate(req); errs != nil {
		response.Fail(c, http.StatusUnprocessableEntity, "validation failed", errs)
		return false
	}
	return true
}

func parseUUID(c *gin.Context) (uuid.UUID, error) {
	return uuid.Parse(c.Param("id"))
}

// canViewAll reports whether the caller may bypass ownership checks on read
// endpoints — i.e. read other users' records. Admin and staff qualify; the
// action itself is already authorized separately by Casbin.
func canViewAll(claims *jwt.Claims) bool {
	return claims != nil && domain.CanViewAllResources(domain.Role(claims.Role))
}

func parseUintParam(c *gin.Context, name string) (uint, error) {
	v, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(v), nil
}
