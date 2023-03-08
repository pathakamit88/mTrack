package middleware

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
)

func BasicAuthorization() gin.HandlerFunc {
	return func(c *gin.Context) {
		f, err := os.ReadFile("authkey.txt")
		if err != nil {
			panic(err)
		}
		if string(f) != c.Request.Header.Get("Authorization") {
			log.Println("Authorization mismatch", c.Request.Header.Get("Authorization"))
			c.Status(http.StatusForbidden)
			c.Abort()
			return
		}
	}
}
