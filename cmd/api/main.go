package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/satyamrajput5/deploydock/internal/config"
	"github.com/satyamrajput5/deploydock/internal/db"
	"github.com/satyamrajput5/deploydock/internal/handlers"
	"github.com/satyamrajput5/deploydock/internal/middleware"
)

func main() {
	log.Println("Starting DeployDock API Server...")

	cfg := config.Load()
	gin.SetMode(cfg.GinMode)

	database := db.Connect(cfg.DatabaseURL)
	defer database.Close()

	if schemaBytes, err := os.ReadFile("migrations/init_schema.sql"); err == nil {
		if err := db.EnsureSchema(database, string(schemaBytes)); err != nil {
			log.Printf("WARN: Automatic schema bootstrap notice: %v", err)
		}
	}

	healthH := handlers.NewHealthHandler(database)
	authH := handlers.NewAuthHandler(database, cfg.JWTSecret, cfg.JWTExpiry)
	deployH := handlers.NewDeploymentHandler(database)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.StaticFile("/", "./dashboard/index.html")
	r.StaticFile("/style.css", "./dashboard/style.css")
	r.StaticFile("/app.js", "./dashboard/app.js")
	r.Static("/static", "./dashboard")

	r.GET("/health", healthH.HealthCheck)

	authRoutes := r.Group("/auth")
	{
		authRoutes.POST("/register", authH.Register)
		authRoutes.POST("/login", authH.Login)
	}

	api := r.Group("/")
	api.Use(middleware.AuthRequired(cfg.JWTSecret))
	{
		api.POST("/deployments", deployH.Create)
		api.GET("/deployments", deployH.List)
		api.GET("/deployments/:id", deployH.Get)
		api.DELETE("/deployments/:id", deployH.Delete)
		api.GET("/deployments/:id/logs", deployH.Logs)
	}

	addr := ":" + cfg.Port
	log.Printf("DeployDock API listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("FATAL: Server crashed: %v", err)
	}
}
