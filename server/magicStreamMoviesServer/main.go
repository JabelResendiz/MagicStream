package main

import (
	"fmt"

	controller "github.com/JabelResendiz/MagicStream/server/magicStreamMoviesServer/controllers"
	"github.com/gin-gonic/gin"
)

func main() {

	// Create a new Gin router with defautl middleware
	router := gin.Default()

	// Endpoint GET /hello
	router.GET("/hello", func(c *gin.Context) {
		c.String(200, "Hello, MagicStreamMovies")
	})

	router.GET("/movies", controller.GetMovies())
	router.GET("/movie/:imdb_id", controller.GetMovie())
	router.POST("/addmovie", controller.AddMovie())
	router.POST("/register", controller.RegisterUser())
	router.POST("/login", controller.LoginUser())

	// Start server on port 8080
	if err := router.Run(":8080"); err != nil {
		fmt.Println("Failed to start server", err)
	}
}
