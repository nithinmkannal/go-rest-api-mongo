package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	mongoURI := envOrDefault("MONGODB_URI", "mongodb://localhost:27017")
	databaseName := envOrDefault("MONGODB_DATABASE", "music")

	connectContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(connectContext, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return fmt.Errorf("connect to MongoDB: %w", err)
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			log.Printf("disconnect from MongoDB: %v", err)
		}
	}()

	if err := client.Ping(connectContext, nil); err != nil {
		return fmt.Errorf("ping MongoDB: %w", err)
	}

	store := NewMongoAlbumStore(client.Database(databaseName).Collection("albums"))
	router := NewRouter(store)

	port := envOrDefault("PORT", "8080")
	log.Printf("server listening on :%s", port)
	if err := router.Run(":" + port); err != nil {
		return fmt.Errorf("run server: %w", err)
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func NewRouter(store AlbumStore) *gin.Engine {
	router := gin.Default()
	router.SetTrustedProxies(nil)
	router.Use(RateLimit(60, time.Minute))

	handler := NewAlbumHandler(store)
	router.GET("/albums", handler.List)
	router.GET("/albums/:id", handler.Get)
	router.POST("/albums", handler.Create)
	router.PUT("/albums/:id", handler.Update)
	router.DELETE("/albums/:id", handler.Delete)
	return router
}
