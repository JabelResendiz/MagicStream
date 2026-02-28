package main

import (
	"fmt"
	"log"

	"github.com/JabelResendiz/MagicStream/server/magicStreamMoviesServer/routes"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {

	// Create a new Gin router with defautl middleware

	err := godotenv.Load(".env")

	if err != nil {
		log.Println("Warning: .env file not found (using system env variables)")
	}

	router := gin.Default()

	// Endpoint GET /hello
	router.GET("/hello", func(c *gin.Context) {
		c.String(200, "Hello, MagicStreamMovies")
	})

	routes.SetupUnProtectedRoutes(router)

	routes.SetupProtectedRoutes(router)

	// Start server on port 8080
	if err := router.Run(":8080"); err != nil {
		fmt.Println("Failed to start server", err)
	}
}
