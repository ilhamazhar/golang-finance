package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func parsePagination(c *gin.Context) (page, limit int) {
	page = 1
	limit = 10

	if v, err := strconv.Atoi(c.DefaultQuery("page", "")); err == nil && v >= 1 {
		page = v
	}
	if v, err := strconv.Atoi(c.DefaultQuery("limit", "")); err == nil && v >= 1 && v <= 100 {
		limit = v
	}
	return
}
