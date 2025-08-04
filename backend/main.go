package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID          int       `json:"id" db:"id"`
	Email       string    `json:"email" db:"email"`
	Username    string    `json:"username" db:"username"`
	Password    string    `json:"-" db:"password_hash"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type SignupRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type Trip struct {
	ID          int       `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	Password    string    `json:"-" db:"password"`
	CreatorID   int       `json:"creator_id" db:"creator_id"`
	Status      string    `json:"status" db:"status"` // created, active, ended
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type CreateTripRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=100"`
	Description string `json:"description" binding:"max=500"`
	Password    string `json:"password" binding:"required,min=4,max=50"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

type Server struct {
	db        *sql.DB
	jwtSecret string
}

func NewServer() *Server {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
		log.Println("Warning: Using default JWT secret. Set JWT_SECRET environment variable in production.")
	}
	return &Server{
		jwtSecret: jwtSecret,
	}
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

func (s *Server) runMigrations() error {
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

	databaseURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	m, err := migrate.New(
		"file://migrations",
		databaseURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	if err == migrate.ErrNoChange {
		log.Println("No new migrations to run")
	} else {
		log.Println("Migrations ran successfully")
	}

	return nil
}

// Password hashing utilities
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// JWT token utilities
func (s *Server) generateTokens(userID int, email string) (string, string, error) {
	// Access token (15 minutes)
	accessClaims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"type":    "access",
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
		"iat":     time.Now().Unix(),
	}
	
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", "", err
	}
	
	// Refresh token (7 days)
	refreshClaims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"type":    "refresh",
		"exp":     time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", "", err
	}
	
	return accessTokenString, refreshTokenString, nil
}

func (s *Server) validateToken(tokenString string) (*jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})
	
	if err != nil {
		return nil, err
	}
	
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return &claims, nil
	}
	
	return nil, fmt.Errorf("invalid token")
}

// User database operations
func (s *Server) createUser(email, password string) (*User, error) {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	
	// Set username to email by default
	username := email
	
	query := `
		INSERT INTO users (email, username, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, email, username, created_at, updated_at
	`
	
	var user User
	err = s.db.QueryRow(query, email, username, hashedPassword).Scan(
		&user.ID, &user.Email, &user.Username, &user.CreatedAt, &user.UpdatedAt,
	)
	
	if err != nil {
		return nil, err
	}
	
	return &user, nil
}

func (s *Server) getUserByEmail(email string) (*User, error) {
	query := `
		SELECT id, email, username, password_hash, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	
	var user User
	err := s.db.QueryRow(query, email).Scan(
		&user.ID, &user.Email, &user.Username, &user.Password, &user.CreatedAt, &user.UpdatedAt,
	)
	
	if err != nil {
		return nil, err
	}
	
	return &user, nil
}

// Trip database operations
func (s *Server) createTrip(name, description, password string, creatorID int) (*Trip, error) {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	
	query := `
		INSERT INTO trips (name, description, password, creator_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'created', NOW(), NOW())
		RETURNING id, name, description, creator_id, status, created_at, updated_at
	`
	
	var trip Trip
	err = s.db.QueryRow(query, name, description, hashedPassword, creatorID).Scan(
		&trip.ID, &trip.Name, &trip.Description, &trip.CreatorID, &trip.Status, &trip.CreatedAt, &trip.UpdatedAt,
	)
	
	if err != nil {
		return nil, err
	}
	
	return &trip, nil
}

func (s *Server) getTripsByUserID(userID int) ([]Trip, error) {
	query := `
		SELECT id, name, description, creator_id, status, created_at, updated_at
		FROM trips
		WHERE creator_id = $1
		ORDER BY created_at DESC
	`
	
	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var trips []Trip
	for rows.Next() {
		var trip Trip
		err := rows.Scan(
			&trip.ID, &trip.Name, &trip.Description, &trip.CreatorID, &trip.Status, &trip.CreatedAt, &trip.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		trips = append(trips, trip)
	}
	
	return trips, nil
}

func (s *Server) getTripByID(tripID int) (*Trip, error) {
	query := `
		SELECT id, name, description, creator_id, status, created_at, updated_at
		FROM trips
		WHERE id = $1
	`
	
	var trip Trip
	err := s.db.QueryRow(query, tripID).Scan(
		&trip.ID, &trip.Name, &trip.Description, &trip.CreatorID, &trip.Status, &trip.CreatedAt, &trip.UpdatedAt,
	)
	
	if err != nil {
		return nil, err
	}
	
	return &trip, nil
}

func (s *Server) startTrip(tripID, userID int) error {
	query := `
		UPDATE trips
		SET status = 'active', updated_at = NOW()
		WHERE id = $1 AND creator_id = $2 AND status = 'created'
	`
	
	result, err := s.db.Exec(query, tripID, userID)
	if err != nil {
		return err
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("trip not found, not authorized, or already started")
	}
	
	return nil
}

func (s *Server) endTrip(tripID, userID int) error {
	query := `
		UPDATE trips
		SET status = 'ended', updated_at = NOW()
		WHERE id = $1 AND creator_id = $2 AND status = 'active'
	`
	
	result, err := s.db.Exec(query, tripID, userID)
	if err != nil {
		return err
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("trip not found or not authorized")
	}
	
	return nil
}

// Input validation utilities
func isValidEmail(email string) bool {
	// Basic email validation - Gin's email binding provides more thorough validation
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email) && len(email) <= 255
}

func isValidPassword(password string) bool {
	return len(password) >= 8
}

// Auth handlers
func (s *Server) handleSignup(c *gin.Context) {
	var req SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Additional validation
	if !isValidEmail(req.Email) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Please provide a valid email address",
		})
		return
	}

	if !isValidPassword(req.Password) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Password must be at least 8 characters long",
		})
		return
	}

	// Check if email already exists
	existingUser, err := s.getUserByEmail(req.Email)
	if err == nil && existingUser != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Email already registered",
		})
		return
	}

	// Create user
	user, err := s.createUser(req.Email, req.Password)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Email already registered",
			})
			return
		}
		
		log.Printf("Error creating user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create user",
		})
		return
	}

	// Generate tokens
	accessToken, refreshToken, err := s.generateTokens(user.ID, user.Email)
	if err != nil {
		log.Printf("Error generating tokens: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate tokens",
		})
		return
	}

	// Set HttpOnly cookies
	c.SetCookie("access_token", accessToken, 15*60, "/", "", false, true) // 15 minutes
	c.SetCookie("refresh_token", refreshToken, 7*24*60*60, "/", "", false, true) // 7 days

	c.JSON(http.StatusCreated, gin.H{
		"user": *user,
		"message": "User created successfully",
	})
}

func (s *Server) handleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Get user by email
	user, err := s.getUserByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid email or password",
		})
		return
	}

	// Check password
	if !checkPasswordHash(req.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid email or password",
		})
		return
	}

	// Generate tokens
	accessToken, refreshToken, err := s.generateTokens(user.ID, user.Email)
	if err != nil {
		log.Printf("Error generating tokens: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate tokens",
		})
		return
	}

	// Clear password from response
	user.Password = ""

	// Set HttpOnly cookies
	c.SetCookie("access_token", accessToken, 15*60, "/", "", false, true) // 15 minutes
	c.SetCookie("refresh_token", refreshToken, 7*24*60*60, "/", "", false, true) // 7 days

	c.JSON(http.StatusOK, gin.H{
		"user": *user,
		"message": "Login successful",
	})
}

func (s *Server) handleLogout(c *gin.Context) {
	// Clear cookies by setting them with negative max age
	c.SetCookie("access_token", "", -1, "/", "", false, true)
	c.SetCookie("refresh_token", "", -1, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"message": "Logout successful",
	})
}

// JWT middleware
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to get access token from cookie
		accessToken, err := c.Cookie("access_token")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Access token required",
			})
			c.Abort()
			return
		}

		// Validate token
		claims, err := s.validateToken(accessToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			c.Abort()
			return
		}

		// Check if it's an access token
		if tokenType, ok := (*claims)["type"].(string); !ok || tokenType != "access" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token type",
			})
			c.Abort()
			return
		}

		// Store user info in context
		c.Set("user_id", (*claims)["user_id"])
		c.Set("email", (*claims)["email"])
		c.Next()
	}
}

// Protected route handlers
func (s *Server) handleGetMe(c *gin.Context) {
	email, exists := c.Get("email")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "User information not found in context",
		})
		return
	}

	user, err := s.getUserByEmail(email.(string))
	if err != nil {
		log.Printf("Error getting user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get user information",
		})
		return
	}

	// Clear password from response
	user.Password = ""

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}

// Trip handlers
func (s *Server) handleCreateTrip(c *gin.Context) {
	var req CreateTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "User information not found",
		})
		return
	}

	// Convert user_id to int (it comes as float64 from JWT claims)
	userIDFloat, ok := userID.(float64)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid user ID format",
		})
		return
	}

	trip, err := s.createTrip(req.Name, req.Description, req.Password, int(userIDFloat))
	if err != nil {
		log.Printf("Error creating trip: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create trip",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"trip": trip,
	})
}

func (s *Server) handleGetTrips(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "User information not found",
		})
		return
	}

	userIDFloat, ok := userID.(float64)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid user ID format",
		})
		return
	}

	trips, err := s.getTripsByUserID(int(userIDFloat))
	if err != nil {
		log.Printf("Error getting trips: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get trips",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"trips": trips,
	})
}

func (s *Server) handleGetTrip(c *gin.Context) {
	tripIDStr := c.Param("id")
	tripID, err := strconv.Atoi(tripIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid trip ID",
		})
		return
	}

	trip, err := s.getTripByID(tripID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Trip not found",
			})
			return
		}
		log.Printf("Error getting trip: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get trip",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"trip": trip,
	})
}

func (s *Server) handleEndTrip(c *gin.Context) {
	tripIDStr := c.Param("id")
	tripID, err := strconv.Atoi(tripIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid trip ID",
		})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "User information not found",
		})
		return
	}

	userIDFloat, ok := userID.(float64)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid user ID format",
		})
		return
	}

	err = s.endTrip(tripID, int(userIDFloat))
	if err != nil {
		if strings.Contains(err.Error(), "not found or not authorized") {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Trip not found or not authorized",
			})
			return
		}
		log.Printf("Error ending trip: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to end trip",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Trip ended successfully",
	})
}

func (s *Server) handleStartTrip(c *gin.Context) {
	tripIDStr := c.Param("id")
	tripID, err := strconv.Atoi(tripIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid trip ID",
		})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "User information not found",
		})
		return
	}

	userIDFloat, ok := userID.(float64)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid user ID format",
		})
		return
	}

	err = s.startTrip(tripID, int(userIDFloat))
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not authorized") || strings.Contains(err.Error(), "already started") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		log.Printf("Error starting trip: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to start trip",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Trip started successfully",
	})
}

func (s *Server) setupRoutes() *gin.Engine {
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://localhost:3000")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token")
		c.Header("Access-Control-Allow-Credentials", "true")

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

		// Authentication routes
		auth := v1.Group("/auth")
		{
			auth.POST("/signup", s.handleSignup)
			auth.POST("/login", s.handleLogin)
			auth.POST("/logout", s.handleLogout)
		}

		// Protected trip routes
		trips := v1.Group("/trips")
		trips.Use(s.authMiddleware())
		{
			trips.POST("", s.handleCreateTrip)
			trips.GET("", s.handleGetTrips)
			trips.GET("/:id", s.handleGetTrip)
			trips.PUT("/:id/start", s.handleStartTrip)
			trips.PUT("/:id/end", s.handleEndTrip)
		}

		// Protected user routes
		users := v1.Group("/users")
		users.Use(s.authMiddleware())
		{
			users.GET("/me", s.handleGetMe)
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
	} else {
		// Run migrations
		if err := server.runMigrations(); err != nil {
			log.Printf("Migration failed: %v", err)
			log.Println("Continuing without running migrations...")
		}
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
