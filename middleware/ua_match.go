package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

func UAMatchInterceptor() gin.HandlerFunc {
	return func(c *gin.Context) {
		userAgent := c.GetHeader("User-Agent")
		rule := operation_setting.MatchUA(userAgent)
		if rule != nil {
			statusCode := rule.StatusCode
			if statusCode <= 0 {
				statusCode = http.StatusOK
			}
			c.String(statusCode, rule.Body)
			c.Abort()
			return
		}
		c.Next()
	}
}
