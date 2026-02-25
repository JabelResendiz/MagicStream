package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
)

func main(){
	
	// Create a new Gin router with defautl middleware
	router:= gin.Default()

	// Endpoint GET /hello
	router.GET("/hello", func(c *gin.Context) {
		c.String(200, "Hello, MagicStreamMovies")
	})

	// Start server on port 8080
	if err:= router.Run(":8080"); err!= nil {
		fmt.Println("Failed to start server", err)
	}
}