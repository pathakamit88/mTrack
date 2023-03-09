package main

import (
	"github.com/gin-gonic/gin"
	"github.com/pathakamit88/mTrack/handler"
	"github.com/pathakamit88/mTrack/middleware"
	"io"
	"log"
	"os"
)

func main() {
	gin.ForceConsoleColor()

	if os.Getenv("GIN_MODE") == "release" {
		f, err := os.OpenFile("mtrack.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		defer f.Close()
		log.SetOutput(f)
		log.SetFlags(log.Lshortfile | log.LstdFlags)

		w, _ := os.Create("requests.log")
		gin.DefaultWriter = io.MultiWriter(w)
	}

	r := gin.Default()

	authorized := r.Group("/", middleware.BasicAuthorization())
	authorized.GET("v1/messages", handler.GetMessages)
	authorized.POST("v1/messages", handler.PostMessage)

	err := r.Run("localhost:8080")
	if err != nil {
		panic(err)
	}
}
