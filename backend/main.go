package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

type Server struct {
	db *sql.DB
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) setupDatabase() error {
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}

	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "keepup"
	}

	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "keepup"
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "keepup"
	}

	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	var err error
	s.db, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	if err = s.db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Successfully connected to database")
	return nil
}

func (s *Server) setupRoutes() *gin.Engine {
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		status := "ok"
		dbStatus := "ok"

		if s.db != nil {
			if err := s.db.Ping(); err != nil {
				dbStatus = "error"
				status = "degraded"
			}
		} else {
			dbStatus = "not_connected"
			status = "degraded"
		}

		c.JSON(http.StatusOK, gin.H{
			"status":   status,
			"database": dbStatus,
			"service":  "keepup-backend",
			"version":  "1.0.0",
		})
	})

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		v1.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "pong",
			})
		})

		// Placeholder for trip routes
		trips := v1.Group("/trips")
		{
			trips.GET("", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"trips": []any{},
				})
			})
		}

		// Placeholder for user routes
		users := v1.Group("/users")
		{
			users.GET("/me", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"user": gin.H{
						"id":   "placeholder",
						"name": "Placeholder User",
					},
				})
			})
		}
	}

	return r
}

func main() {
	server := NewServer()

	// Setup database connection
	if err := server.setupDatabase(); err != nil {
		log.Printf("Database connection failed: %v", err)
		log.Println("Continuing without database...")
	}
	defer func() {
		if server.db != nil {
			server.db.Close()
		}
	}()

	// Setup routes
	router := server.setupRoutes()

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
