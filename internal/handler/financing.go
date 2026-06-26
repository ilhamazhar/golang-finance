package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ilhamazhar/golang-gpt/internal/domain"
	"github.com/ilhamazhar/golang-gpt/internal/middleware"
	"github.com/ilhamazhar/golang-gpt/pkg/response"
)

type FinancingHandler struct {
	svc domain.FinancingService
}

func NewFinancingHandler(svc domain.FinancingService) *FinancingHandler {
	return &FinancingHandler{svc: svc}
}

func (h *FinancingHandler) Create(c *gin.Context) {
	claims := middleware.ClaimsFromContext(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	var req domain.CreateMurabahahRequest
	if !bindJSON(c, &req) {
		return
	}

	result, err := h.svc.CreateMurabahah(c.Request.Context(), claims.UserID, req)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	response.OK(c, http.StatusCreated, "Financing created", result)
}

func (h *FinancingHandler) GetByID(c *gin.Context) {
	claims := middleware.ClaimsFromContext(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id", nil)
		return
	}

	result, err := h.svc.GetByID(c.Request.Context(), claims.UserID, id, canViewAll(claims))
	if err != nil {
		response.Fail(c, http.StatusNotFound, err.Error(), nil)
		return
	}
	response.OK(c, http.StatusOK, "Financing retrieved", result)
}

func (h *FinancingHandler) Sign(c *gin.Context) {
	claims := middleware.ClaimsFromContext(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id", nil)
		return
	}

	result, err := h.svc.SignAkad(c.Request.Context(), claims.UserID, id)
	if err != nil {
		writeFinancingError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Akad signed", result)
}

func (h *FinancingHandler) PayInstallment(c *gin.Context) {
	claims := middleware.ClaimsFromContext(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id", nil)
		return
	}
	no, err := strconv.Atoi(c.Param("no"))
	if err != nil || no < 1 {
		response.Fail(c, http.StatusBadRequest, "invalid installment number", nil)
		return
	}

	result, err := h.svc.PayInstallment(c.Request.Context(), claims.UserID, id, no)
	if err != nil {
		writeFinancingError(c, err)
		return
	}
	response.OK(c, http.StatusCreated, "Installment payment created", result)
}

// writeFinancingError maps financing domain errors to HTTP status codes.
// Anything unrecognised after validation is a downstream (payment/Xendit)
// failure, surfaced as 502.
func writeFinancingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		response.Fail(c, http.StatusNotFound, "financing not found", nil)
	case errors.Is(err, domain.ErrFinancingNotDraft),
		errors.Is(err, domain.ErrFinancingNotActive),
		errors.Is(err, domain.ErrInstallmentPaid):
		response.Fail(c, http.StatusConflict, err.Error(), nil)
	default:
		response.Fail(c, http.StatusBadGateway, err.Error(), nil)
	}
}

func (h *FinancingHandler) List(c *gin.Context) {
	claims := middleware.ClaimsFromContext(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	page, limit := parsePagination(c)
	search := strings.TrimSpace(c.Query("search"))
	sort := c.Query("sort")
	order := c.Query("order")

	result, total, err := h.svc.List(c.Request.Context(), claims.UserID, page, limit, search, sort, order, canViewAll(claims))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	pagination := response.NewPagination(page, limit, total)
	response.OKPaginated(c, http.StatusOK, "Financings retrieved", result, pagination)
}
