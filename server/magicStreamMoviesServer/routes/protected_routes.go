package routes

import (
	controller "github.com/JabelResendiz/MagicStream/server/magicStreamMoviesServer/controllers"
	"github.com/JabelResendiz/MagicStream/server/magicStreamMoviesServer/middleware"
	"github.com/gin-gonic/gin"
)

func SetupProtectedRoutes(router *gin.Engine) {
	router.Use(middleware.AuthMiddleware())

	router.GET("/movie/:imdb_id", controller.GetMovie())
	router.POST("/addmovie", controller.AddMovie())
	router.GET("/recommendedmovies", controller.GetRecommendedMovies())
	router.PATCH("/updatereview/:imdb_id", controller.AdminReviewUpdate())
}
