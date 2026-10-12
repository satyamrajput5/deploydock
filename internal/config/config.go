package config

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	JWTExpiry   time.Duration
	GinMode     string
}

func Load() *Config {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://deploydock:deploydock@localhost:5432/deploydock?sslmode=disable"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		log.Fatalf("FATAL: JWT_SECRET environment variable must be at least 32 characters long (found %d chars)", len(jwtSecret))
	}

	expiryStr := os.Getenv("JWT_EXPIRY")
	if expiryStr == "" {
		expiryStr = "24h"
	}
	expiry, err := time.ParseDuration(expiryStr)
	if err != nil {
		log.Printf("WARN: Invalid JWT_EXPIRY '%s', defaulting to 24h: %v", expiryStr, err)
		expiry = 24 * time.Hour
	}

	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		ginMode = "debug"
	}

	return &Config{
		Port:        port,
		DatabaseURL: dbURL,
		JWTSecret:   jwtSecret,
		JWTExpiry:   expiry,
		GinMode:     ginMode,
	}
}
