package handler

import (
	stderrors "errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"bank-account/dto"
	apperrors "bank-account/errors"
	"bank-account/service"
)

type Handler struct {
	svc *service.Service
}

func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

type envelope struct {
	Data  interface{} `json:"data"`
	Error *string     `json:"error"`
}

func writeSuccess(c *gin.Context, status int, data interface{}) {
	c.JSON(status, envelope{Data: data, Error: nil})
}

func writeError(c *gin.Context, err error) {
	code := errToCode(err)
	c.JSON(errToStatus(err), envelope{Data: nil, Error: &code})
}

func errToCode(err error) string {
	switch {
	case stderrors.Is(err, apperrors.ErrAccountNotFound):
		return "account_not_found"
	case stderrors.Is(err, apperrors.ErrAccountClosed):
		return "account_closed"
	case stderrors.Is(err, apperrors.ErrAlreadyClosed):
		return "already_closed"
	case stderrors.Is(err, apperrors.ErrInsufficientFunds):
		return "insufficient_funds"
	case stderrors.Is(err, apperrors.ErrInvalidAmount):
		return "invalid_amount"
	case stderrors.Is(err, apperrors.ErrInvalidHolder):
		return "invalid_holder"
	case stderrors.Is(err, apperrors.ErrInvalidAccountType):
		return "invalid_account_type"
	case stderrors.Is(err, apperrors.ErrDailyLimitExceeded):
		return "daily_limit_exceeded"
	default:
		return "internal_error"
	}
}

func errToStatus(err error) int {
	switch {
	case stderrors.Is(err, apperrors.ErrAccountNotFound):
		return http.StatusNotFound
	case stderrors.Is(err, apperrors.ErrAccountClosed),
		stderrors.Is(err, apperrors.ErrAlreadyClosed):
		return http.StatusConflict
	case stderrors.Is(err, apperrors.ErrInsufficientFunds),
		stderrors.Is(err, apperrors.ErrDailyLimitExceeded):
		return http.StatusUnprocessableEntity
	case stderrors.Is(err, apperrors.ErrInvalidAmount),
		stderrors.Is(err, apperrors.ErrInvalidHolder),
		stderrors.Is(err, apperrors.ErrInvalidAccountType):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func (h *Handler) CreateAccount(c *gin.Context) {
	var req dto.CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, apperrors.ErrInvalidHolder)
		return
	}
	info, err := h.svc.CreateAccount(req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeSuccess(c, http.StatusCreated, info)
}

func (h *Handler) GetAccount(c *gin.Context) {
	info, err := h.svc.GetAccount(dto.GetAccountRequest{
		AccountID: c.Param("id"),
	})
	if err != nil {
		writeError(c, err)
		return
	}
	writeSuccess(c, http.StatusOK, info)
}
