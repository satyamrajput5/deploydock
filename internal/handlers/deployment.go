package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/satyamrajput5/deploydock/internal/models"
)

var githubURLRegex = regexp.MustCompile(`^https://github\.com/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+(\.git)?$`)

type DeploymentHandler struct {
	db *sqlx.DB
}

func NewDeploymentHandler(db *sqlx.DB) *DeploymentHandler {
	return &DeploymentHandler{db: db}
}

func getUserID(c *gin.Context) *int {
	val, exists := c.Get("userID")
	if !exists {
		return nil
	}
	id, ok := val.(int)
	if !ok {
		return nil
	}
	return &id
}

func (h *DeploymentHandler) Create(c *gin.Context) {
	var req models.CreateDeploymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	if !githubURLRegex.MatchString(req.RepoURL) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid GitHub repository URL. Must match format: https://github.com/owner/repository",
		})
		return
	}

	userID := getUserID(c)

	var deployment models.Deployment
	insertQuery := `
		INSERT INTO public.deployments (user_id, repo_url, status, created_at, updated_at)
		VALUES ($1, $2, 'pending', NOW(), NOW())
		RETURNING id, user_id, repo_url, status, port, container_id, image_tag, url, error_message, build_number, created_at, updated_at
	`
	err := h.db.QueryRowxContext(c.Request.Context(), insertQuery, userID, req.RepoURL).StructScan(&deployment)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create deployment record: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, deployment)
}

func (h *DeploymentHandler) List(c *gin.Context) {
	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	deployments := make([]models.Deployment, 0)
	query := `
		SELECT id, user_id, repo_url, status, port, container_id, image_tag, url, error_message, build_number, created_at, updated_at
		FROM public.deployments
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	err := h.db.SelectContext(c.Request.Context(), &deployments, query, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query deployments: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deployments": deployments,
		"limit":       limit,
		"offset":      offset,
		"count":       len(deployments),
	})
}

func (h *DeploymentHandler) Get(c *gin.Context) {
	deploymentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid deployment ID format"})
		return
	}

	var deployment models.Deployment
	query := `
		SELECT id, user_id, repo_url, status, port, container_id, image_tag, url, error_message, build_number, created_at, updated_at
		FROM public.deployments
		WHERE id = $1
	`
	err = h.db.GetContext(c.Request.Context(), &deployment, query, deploymentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, deployment)
}

func (h *DeploymentHandler) Delete(c *gin.Context) {
	deploymentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid deployment ID format"})
		return
	}

	query := `DELETE FROM public.deployments WHERE id = $1`
	result, err := h.db.ExecContext(c.Request.Context(), query, deploymentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete deployment: " + err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Deployment deleted successfully",
		"id":      deploymentID,
	})
}

func (h *DeploymentHandler) Logs(c *gin.Context) {
	deploymentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid deployment ID format"})
		return
	}

	logs := make([]models.DeploymentLog, 0)
	query := `
		SELECT id, deployment_id, log_type, stream, message, created_at
		FROM public.deployment_logs
		WHERE deployment_id = $1
		ORDER BY created_at ASC
	`
	if err := h.db.SelectContext(c.Request.Context(), &logs, query, deploymentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve logs: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deployment_id": deploymentID,
		"logs":          logs,
	})
}
