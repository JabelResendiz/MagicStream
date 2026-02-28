package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/JabelResendiz/MagicStream/server/magicStreamMoviesServer/database"
	"github.com/JabelResendiz/MagicStream/server/magicStreamMoviesServer/models"
	"github.com/JabelResendiz/MagicStream/server/magicStreamMoviesServer/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/tmc/langchaingo/llms/openai"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
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
	// err = godotenv.Load(".env")

	// if err != nil {
	// 	log.Println("Warning: .env file not found")
	// }

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

// GetRecommendedMovies return a Gin handler that provides movie recommendations
// based on the authenticated user's favourite genres.
func GetRecommendedMovies() gin.HandlerFunc {
	return func(c *gin.Context) {

		// retrieve the authenticated userId from the Gin context
		userId, err := utils.GetUserIdFromContext(c)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User id not found int this context"})
			return
		}

		// Fetch the user's favourite genres from the database
		favourite_genres, err := GetUsersFavouriteGenres(userId)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Load env variables from .env file
		//err = godotenv.Load(".env")

		// if err != nil {
		// 	log.Println("Warning: .env file not found")
		// }

		// Set a default limit for recommended movies
		var recommendedMovieLimitVal int64 = 5

		// retrieve the limit value from env variables
		// recommendedMovieLimitStr := os.Getenv("RECOMMENDED_MOVIE_LIMIT")

		// if recommendedMovieLimitStr != "" {
		// 	recommendedMovieLimitVal, _ = strconv.ParseInt(recommendedMovieLimitStr, 10, 64)
		// }
		if val := os.Getenv("RECOMMENDED_MOVIE_LIMIT"); val != "" {
			parsed, err := strconv.ParseInt(val, 10, 64)
			if err == nil {
				recommendedMovieLimitVal = parsed
			}
		}

		// Create MongoDB find options
		findOptions := options.Find()

		// Sort results by ranking value in ascending order
		findOptions.SetSort(bson.D{{Key: "ranking.ranking_value", Value: 1}})

		// Limit the number of returned documents
		findOptions.SetLimit(recommendedMovieLimitVal)

		// Filter movies whose genre name is within the user's faavourite genres
		filter := bson.M{"genre.genre_name": bson.M{"$in": favourite_genres}}

		// Create a context with timeout to avoid long running queries
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		defer cancel()

		// Execute the mongoDB query
		cursor, err := movieCollection.Find(ctx, filter, findOptions)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching recommended movies"})
			return
		}

		defer cursor.Close(ctx)

		var recommendedMovies []models.Movie

		// Decode all documents from the cursor into the slice
		if err := cursor.All(ctx, &recommendedMovies); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// return the recommended movies as a JSON response
		c.JSON(http.StatusOK, recommendedMovies)
	}
}

// retrieves a list of favourite genre name for a given user ID
func GetUsersFavouriteGenres(userId string) ([]string, error) {
	var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	// define a filter to find the user by user_id
	filter := bson.M{"user_id": userId}

	// define ea projection to only return favourite_genre.genre_nmae field
	projection := bson.M{
		"favourite_genres.genre_name": 1,
		"_id":                         0,
	}

	// Apply projection to the FindONe optiosn
	opts := options.FindOne().SetProjection(projection)

	// Create a generic BSON map to store the result
	var result bson.M

	// Execute the query and decode the result
	err := userCollection.FindOne(ctx, filter, opts).Decode(&result)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return []string{}, nil
		}
	}

	// Extract favourite_genres as a BSON array
	favGenresArray, ok := result["favourite_genres"].(bson.A)

	if !ok {
		return []string{}, errors.New("unable to retrieve favourite genres for user")
	}

	var genreNames []string

	// iterate over each genre item in the array
	for _, item := range favGenresArray {

		// Convert each item into BSON document
		if genreMap, ok := item.(bson.D); ok {

			//Iterate over fiedls inside the document
			for _, elem := range genreMap {

				// check if the field is "genre_name"
				if elem.Key == "genre_name" {
					// Assert the value as string and append it
					if name, ok := elem.Value.(string); ok {
						genreNames = append(genreNames, name)
					}
				}
			}
		}
	}

	// return the extracted genre names
	return genreNames, nil
}
