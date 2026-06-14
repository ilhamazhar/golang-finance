package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

func parseID(c *gin.Context) (uint, error) {
	n, err := strconv.ParseUint(c.Param("id"), 10, 64)
	return uint(n), err
}

func parseUUID(c *gin.Context) (uuid.UUID, error) {
	return uuid.Parse(c.Param("id"))
}
