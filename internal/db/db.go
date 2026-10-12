package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func Connect(databaseURL string) *sqlx.DB {
	db, err := sqlx.Open("postgres", databaseURL)
	if err != nil {
		log.Fatalf("FATAL: Failed to initialize database driver: %v", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("FATAL: Unable to reach PostgreSQL database at %s: %v", databaseURL, err)
	}

	log.Println("SUCCESS: Successfully connected to PostgreSQL database")
	return db
}

func EnsureSchema(db *sqlx.DB, schemaSQL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, schemaSQL)
	if err != nil {
		return fmt.Errorf("failed executing schema: %w", err)
	}
	log.Println("SUCCESS: Verified/applied database schema")
	return nil
}
