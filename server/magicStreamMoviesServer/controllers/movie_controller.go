package controllers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/JabelResendiz/MagicStream/server/magicStreamMoviesServer/database"
	"github.com/JabelResendiz/MagicStream/server/magicStreamMoviesServer/models"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/tmc/langchaingo/llms/openai"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var movieCollection *mongo.Collection = database.OpenCollection("movies")
var rankingCollection *mongo.Collection = database.OpenCollection("rankings")
var validate = validator.New()

// GetMovies handles GET /movies requests
// It retrieves all movies from the MongoDb collection and returns them as JSON.
func GetMovies() gin.HandlerFunc {
	return func(c *gin.Context) {

		// Create a context with timeout to aoivd long-running database operatiosn
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)

		defer cancel()

		var movies []models.Movie

		// Find all documents in the collection (empty filter)
		cursor, err := movieCollection.Find(ctx, bson.M{})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch movies."})
		}

		defer cursor.Close(ctx)

		// Decode all documents into the movies slice
		if err = cursor.All(ctx, &movies); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode movies."})
		}

		// Return the movies as a JSON response
		c.JSON(http.StatusOK, movies)

	}
}

func GetMovie() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)

		defer cancel()

		movieID := c.Param("imdb_id")

		if movieID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Movie ID is required"})
			return
		}

		var movie models.Movie

		err := movieCollection.FindOne(ctx, bson.M{"imdb_id": movieID}).Decode(&movie)

		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Movie not found"})
			return
		}

		c.JSON(http.StatusOK, movie)

	}
}

func AddMovie() gin.HandlerFunc {
	return func(c *gin.Context) {

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)

		defer cancel()

		var movie models.Movie

		if err := c.ShouldBindJSON(&movie); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		if err := validate.Struct(movie); err != nil {
			var errors []string
			for _, err := range err.(validator.ValidationErrors) {
				errors = append(errors, err.Field()+" failed on "+err.Tag())
			}

			c.JSON(http.StatusBadRequest, gin.H{
				"errors": errors,
			})
			return
		}

		result, err := movieCollection.InsertOne(ctx, movie)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add movie"})
			return
		}

		c.JSON(http.StatusCreated, result)

	}
}

// AdminReviewUpdate handles updating an admin review and generating its ranking using AI
func AdminReviewUpdate() gin.HandlerFunc {
	return func(c *gin.Context) {
		movieId := c.Param("imdb_id")

		if movieId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Movie Id is required"})
			return
		}

		// Define request body structure to bind incoming JSON.
		var req struct {
			AdminReview string `json:"admin_review"`
		}

		// Define request body structure to bind incoming JSON.
		var resp struct {
			RankingName string `json:"rankin_name"`
			AdminReview string `json:"admin_review"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		// Define response structure to return ranking and review.
		sentiment, rankVal, err := GetReviewRanking(req.AdminReview)

		// if err != nil {
		// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "ERROR getting review ranking"})
		// 	return
		// }

		if err != nil {
			fmt.Println("ERROR:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Create MongoDB filter to find the movie by imdb_id.
		filter := bson.M{"imdb_id": movieId}

		// Create context with timeout to prevent long-running DB operations.
		update := bson.M{
			"$set": bson.M{
				"admin_review": req.AdminReview,
				"ranking": bson.M{
					"ranking_value": rankVal,
					"ranking_name":  sentiment,
				},
			},
		}

		// Create context with timeout to prevent long-running DB operations.
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		defer cancel()

		// Execute MongoDB update operation.
		result, err := movieCollection.UpdateOne(ctx, filter, update)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating movie"})
			return
		}

		if result.MatchedCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Movie not found"})
			return
		}

		resp.RankingName = sentiment
		resp.AdminReview = req.AdminReview

		c.JSON(http.StatusOK, resp)

	}
}

// GetReviewRanking determines the ranking category and numeric value using AI.
func GetReviewRanking(admin_review string) (string, int, error) {

	// Retrieve all ranking definitions from database.
	rankings, err := GetRankings()

	if err != nil {
		return "", 0, err
	}

	sentimentDelimited := ""

	for _, ranking := range rankings {
		// Exclude special ranking value (e.g., 999 used as placeholder).
		if ranking.RankingValue != 999 {
			sentimentDelimited = sentimentDelimited + ranking.RankingName + ","
		}
	}

	// remove trailing comma
	sentimentDelimited = strings.Trim(sentimentDelimited, ",")

	// Load env variables and read OpenAI API key
	err = godotenv.Load(".env")

	if err != nil {
		log.Println("Warning: .env file not found")
	}

	OpenAIApiKey := os.Getenv("OPENAI_API_KEY")

	if OpenAIApiKey == "" {
		return "", 0, errors.New("could not read OPENAI_API_KEY")
	}

	// Initialize OpenAI client
	llm, err := openai.New(openai.WithToken(OpenAIApiKey))

	if err != nil {
		return "", 0, err
	}

	// Retrieve base promtp template form env
	base_prompt_template := os.Getenv("BASE_PROMPT_TEMPLATE")

	// Replace placeholder with actual ranking list
	base_prompt := strings.Replace(base_prompt_template, "{rankings}", sentimentDelimited, 1)

	// Call OpenAI model with constructed prompt
	response, err := llm.Call(context.Background(), base_prompt+admin_review)

	if err != nil {
		return "", 0, err
	}

	// Initialize ranking value
	rankVal := 0

	// Map AI response to its numeric ranking value
	for _, ranking := range rankings {
		if ranking.RankingName == response {
			rankVal = ranking.RankingValue
			break
		}
	}

	// Return ranking name and numeric value
	return response, rankVal, nil

}

// GetRankings retrieves all ranking records from MongoDB
func GetRankings() ([]models.Ranking, error) {
	var rankings []models.Ranking

	var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	// Query MongoDB collection without filters
	cursor, err := rankingCollection.Find(ctx, bson.M{})

	if err != nil {
		return nil, err
	}

	defer cursor.Close(ctx)

	// decode all documents into rankings slice
	if err := cursor.All(ctx, &rankings); err != nil {
		return nil, err
	}

	return rankings, nil
}
