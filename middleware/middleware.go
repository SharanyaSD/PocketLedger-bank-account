package middleware

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"
const requestIDKey = "request_id"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(RequestIDHeader)
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Writer.Header().Set(RequestIDHeader, rid)
		c.Set(requestIDKey, rid)
		c.Next()
	}
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		rid, _ := c.Get(requestIDKey)
		log.Printf("rid=%v method=%s path=%s status=%d latency=%s",
			rid, c.Request.Method, c.Request.URL.Path,
			c.Writer.Status(), time.Since(start))
	}
}

func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		rid, _ := c.Get(requestIDKey)
		log.Printf("rid=%v panic=%v path=%s", rid, recovered, c.Request.URL.Path)
		code := "internal_error"
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"data":  nil,
			"error": code,
		})
	})
}
