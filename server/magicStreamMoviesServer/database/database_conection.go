package database

import(
	"fmt"
	"log"
	"os"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// DBInstance creates and return MongoDB client instance
// It loads env variables and establishes the database connection.
func DBInstance() *mongo.Client{

	// Load env variables from env file
	err := godotenv.Load(".env")

	if err != nil {
		log.Println("Warning: unable to find .env file")
	}

	// Get MongoDB connection URI from env variables
	MongoDb := os.Getenv("MONGODB_URI")

	if MongoDb == "" {
		log.Fatal("MONGO_URI not set!")
	}

	fmt.Println("MongoDB  URI: ", MongoDb)

	// Apply the MongoDB connection URI to client options
	clientOptions := options.Client().ApplyURI(MongoDb)

	// Establish connection to MongoDB
	client, err := mongo.Connect(clientOptions)

	if err != nil {
		return nil
	}

	return client
}

var Client *mongo.Client = DBInstance()


// OpenCollection returns a reference to specific MongoDB collection
// It reads the database name from env variables
func OpenCollection(collectionName string) *mongo.Collection {
	err := godotenv.Load(".env")

	if err != nil {
		log.Println("Warning: unable to find .env file")
	}

	// Get database name from env variables
	databaseName := os.Getenv("DATABASE_NAME")
	if err =  cursor.All(ctx, &movies); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error":"Failed to decode movies."})
	}

	fmt.Println("DATABASE_NAME: ", databaseName)

	// Return the requested collection
	collection := Client.Database(databaseName).Collection(collectionName)

	if collection == nil {
		return nil
	}

	return collection
}