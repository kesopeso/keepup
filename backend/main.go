package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
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

	c.JSON(http.StatusCreated, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         *user,
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

	c.JSON(http.StatusOK, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         *user,
	})
}

// JWT middleware
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})
			c.Abort()
			return
		}

		// Check if header starts with "Bearer "
		tokenString := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = authHeader[7:]
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization header format",
			})
			c.Abort()
			return
		}

		// Validate token
		claims, err := s.validateToken(tokenString)
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

		// Authentication routes
		auth := v1.Group("/auth")
		{
			auth.POST("/signup", s.handleSignup)
			auth.POST("/login", s.handleLogin)
		}

		// Placeholder for trip routes
		trips := v1.Group("/trips")
		{
			trips.GET("", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"trips": []any{},
				})
			})
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
