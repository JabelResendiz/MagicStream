package controllers

import(
	"context"
	"time"
	"net/http"
	"github.com/JabelResendiz/MagicStream/server/magicStreamMoviesServer/models"
	"github.com/JabelResendiz/MagicStream/server/magicStreamMoviesServer/database"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var movieCollection *mongo.Collection = database.OpenCollection("movies")

// GetMovies handles GET /movies requests
// It retrieves all movies from the MongoDb collection and returns them as JSON.
func GetMovies() gin.HandlerFunc {
	return func(c *gin.Context){
		
		// Create a context with timeout to aoivd long-running database operatiosn
		ctx,cancel := context.WithTimeout(context.Background(), 100*time.Second)

		defer cancel()

		var movies[]models.Movie

		// Find all documents in the collection (empty filter)
		cursor, err := movieCollection.Find(ctx, bson.M{})

		if err != nil  {
			c.JSON(http.StatusInternalServerError, gin.H{"error":"Failed to fetch movies."})
		}

		defer cursor.Close(ctx)

		// Decode all documents into the movies slice
		if err =  cursor.All(ctx, &movies); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error":"Failed to decode movies."})
		}

		// Return the movies as a JSON response
		c.JSON(http.StatusOK, movies)


	}
}

