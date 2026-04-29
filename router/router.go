package router

import (
	"github.com/gin-gonic/gin"

	"bank-account/handler"
	"bank-account/middleware"
)

func New(h *handler.Handler) *gin.Engine {
	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())

	v1 := r.Group("/api/v1")
	{
		v1.POST("/accounts", h.CreateAccount)
		v1.GET("/accounts/:id", h.GetAccount)
		v1.POST("/accounts/:id/deposit", h.Deposit)
	}
	return r
}
