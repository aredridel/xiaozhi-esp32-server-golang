package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// InternalServiceAuth internal service authentication middleware.
// Requires: Authorization: Bearer <token>
func InternalServiceAuth(expectedToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing internal service authentication info"})
			c.Abort()
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if token == "" || token != expectedToken {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid internal service authentication info"})
			c.Abort()
			return
		}

		c.Next()
	}
}
