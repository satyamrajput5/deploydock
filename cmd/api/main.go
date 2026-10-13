package main

import (
	"context"
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/satyamrajput5/deploydock/internal/config"
	"github.com/satyamrajput5/deploydock/internal/db"
	"github.com/satyamrajput5/deploydock/internal/handlers"
	"github.com/satyamrajput5/deploydock/internal/worker"
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

	workerPool := worker.NewWorkerPool(database, 100)
	workerPool.Start(context.Background(), 4)

	healthH := handlers.NewHealthHandler(database)
	authH := handlers.NewAuthHandler(database, cfg.JWTSecret, cfg.JWTExpiry)
	deployH := handlers.NewDeploymentHandler(database)
	webhookH := handlers.NewWebhookHandler(database)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.StaticFile("/", "./dashboard/index.html")
	r.StaticFile("/style.css", "./dashboard/style.css")
	r.StaticFile("/app.js", "./dashboard/app.js")
	r.Static("/static", "./dashboard")
	r.GET("/deployment/:id", func(c *gin.Context) {
		c.File("./dashboard/index.html")
	})

	r.GET("/health", healthH.HealthCheck)

	authRoutes := r.Group("/auth")
	{
		authRoutes.POST("/register", authH.Register)
		authRoutes.POST("/login", authH.Login)
	}

	r.POST("/webhooks/github/:deploymentId", webhookH.HandleGitHub)

	api := r.Group("/")
	{
		api.POST("/deployments", deployH.Create)
		api.GET("/deployments", deployH.List)
		api.GET("/deployments/:id", deployH.Get)
		api.PATCH("/deployments/:id/config", deployH.UpdateConfig)
		api.POST("/deployments/:id/redeploy", deployH.Redeploy)
		api.GET("/deployments/:id/builds", deployH.ListBuilds)
		api.GET("/deployments/:id/builds/:buildId", deployH.GetBuild)
		api.POST("/deployments/:id/rollback", deployH.Rollback)
		api.DELETE("/deployments/:id", deployH.Delete)
		api.GET("/deployments/:id/logs", deployH.Logs)
		api.POST("/repos/inspect", deployH.InspectRepo)
		api.GET("/deployments/:id/env-vars", deployH.GetEnvVars)
		api.PUT("/deployments/:id/env-vars", deployH.UpdateEnvVars)
	}

	addr := ":" + cfg.Port
	log.Printf("DeployDock API listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("FATAL: Server crashed: %v", err)
	}
}
