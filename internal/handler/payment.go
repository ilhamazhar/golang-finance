package handler

import (
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ilhamazhar/golang-gpt/internal/domain"
	"github.com/ilhamazhar/golang-gpt/internal/middleware"
	"github.com/ilhamazhar/golang-gpt/pkg/response"
)

const maxWebhookBodyBytes = 1 << 20 // 1 MB

type PaymentHandler struct {
	svc domain.PaymentService
}

func NewPaymentHandler(svc domain.PaymentService) *PaymentHandler {
	return &PaymentHandler{svc: svc}
}

func (h *PaymentHandler) CreateQRIS(c *gin.Context) {
	claims := middleware.ClaimsFromContext(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	var req domain.CreateQRISRequest
	if !bindJSON(c, &req) {
		return
	}

	result, err := h.svc.CreateQRIS(c.Request.Context(), claims.UserID, req)
	if err != nil {
		response.Fail(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	response.OK(c, http.StatusCreated, "QRIS created", result)
}

func (h *PaymentHandler) GetStatus(c *gin.Context) {
	claims := middleware.ClaimsFromContext(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	orderRef := c.Param("order_ref")
	result, err := h.svc.GetStatus(c.Request.Context(), claims.UserID, orderRef)
	if err != nil {
		response.Fail(c, http.StatusNotFound, err.Error(), nil)
		return
	}
	response.OK(c, http.StatusOK, "Order status retrieved", result)
}

// Webhook receives Xendit's callback. Public endpoint, authed via X-Callback-Token header.
func (h *PaymentHandler) Webhook(c *gin.Context) {
	callbackToken := c.GetHeader("x-callback-token")

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxWebhookBodyBytes)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.Status(http.StatusRequestEntityTooLarge)
		return
	}

	if err := h.svc.HandleWebhook(c.Request.Context(), callbackToken, body); err != nil {
		log.Printf("webhook error: %v", err)
	}
	c.Status(http.StatusOK)
}
