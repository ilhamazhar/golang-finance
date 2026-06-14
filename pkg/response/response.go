package response

import "github.com/gin-gonic/gin"

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
	Errors  any    `json:"errors,omitempty"`
}

type Pagination struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

type PaginatedResponse struct {
	Success    bool       `json:"success"`
	Message    string     `json:"message,omitempty"`
	Data       any        `json:"data,omitempty"`
	Pagination Pagination `json:"pagination"`
}

func NewPagination(page, limit int, totalItems int64) Pagination {
	totalPages := int(totalItems) / limit
	if int(totalItems)%limit != 0 {
		totalPages++
	}
	return Pagination{
		Page:       page,
		Limit:      limit,
		TotalItems: totalItems,
		TotalPages: totalPages,
	}
}

func OK(c *gin.Context, status int, message string, data any) {
	c.JSON(status, Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func OKPaginated(c *gin.Context, status int, message string, data any, pagination Pagination) {
	c.JSON(status, PaginatedResponse{
		Success:    true,
		Message:    message,
		Data:       data,
		Pagination: pagination,
	})
}

func Fail(c *gin.Context, status int, message string, errors any) {
	c.JSON(status, Response{
		Success: false,
		Message: message,
		Errors:  errors,
	})
}
