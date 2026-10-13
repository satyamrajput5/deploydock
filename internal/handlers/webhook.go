package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/satyamrajput5/deploydock/internal/crypto"
	"github.com/satyamrajput5/deploydock/internal/models"
	"github.com/satyamrajput5/deploydock/internal/worker"
)

type WebhookHandler struct {
	db *sqlx.DB
}

func NewWebhookHandler(db *sqlx.DB) *WebhookHandler {
	return &WebhookHandler{db: db}
}

type githubPushPayload struct {
	Ref        string `json:"ref"`
	After      string `json:"after"`
	Before     string `json:"before"`
	Deleted    bool   `json:"deleted"`
	Repository struct {
		CloneURL string `json:"clone_url"`
		HTMLURL  string `json:"html_url"`
	} `json:"repository"`
}

func (h *WebhookHandler) HandleGitHub(c *gin.Context) {
	deploymentID, err := strconv.Atoi(c.Param("deploymentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid deployment ID"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read payload body"})
		return
	}

	var deployment models.Deployment
	query := `
		SELECT id, repo_url, branch, auto_deploy, webhook_id, webhook_secret_enc
		FROM public.deployments
		WHERE id = $1
	`
	if err := h.db.GetContext(c.Request.Context(), &deployment, query, deploymentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
		return
	}

	sig := c.GetHeader("X-Hub-Signature-256")
	if deployment.WebhookSecretEnc != nil && *deployment.WebhookSecretEnc != "" {
		secret, err := crypto.Decrypt(*deployment.WebhookSecretEnc)
		if err == nil && secret != "" {
			if !crypto.VerifyHubSignature(body, secret, sig) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid webhook signature"})
				return
			}
		}
	}

	deliveryID := c.GetHeader("X-GitHub-Delivery")
	if deliveryID != "" {
		var exists bool
		_ = h.db.GetContext(c.Request.Context(), &exists, `SELECT EXISTS(SELECT 1 FROM public.webhook_deliveries WHERE delivery_id = $1)`, deliveryID)
		if exists {
			c.JSON(http.StatusOK, gin.H{"message": "Delivery already processed"})
			return
		}
	}

	eventType := c.GetHeader("X-GitHub-Event")
	if eventType != "push" {
		c.JSON(http.StatusOK, gin.H{"message": "Ignored non-push event: " + eventType})
		return
	}

	var payload githubPushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Malformed JSON payload"})
		return
	}

	if payload.Deleted {
		c.JSON(http.StatusOK, gin.H{"message": "Ignored branch deletion"})
		return
	}

	expectedRef := "refs/heads/" + deployment.Branch
	if payload.Ref != expectedRef && !strings.HasSuffix(payload.Ref, "/"+deployment.Branch) {
		c.JSON(http.StatusOK, gin.H{"message": "Ignored push to non-target branch: " + payload.Ref})
		return
	}

	if deliveryID != "" {
		insertDelivery := `
			INSERT INTO public.webhook_deliveries (delivery_id, deployment_id, event_type, commit_sha, processing_status, received_at)
			VALUES ($1, $2, $3, $4, 'accepted', NOW())
			ON CONFLICT (delivery_id) DO NOTHING
		`
		_, _ = h.db.ExecContext(c.Request.Context(), insertDelivery, deliveryID, deployment.ID, eventType, payload.After)
	}

	if !deployment.AutoDeploy {
		c.JSON(http.StatusOK, gin.H{"message": "Push received but auto-deploy is disabled"})
		return
	}

	var newBuild models.DeploymentBuild
	insertBuild := `
		INSERT INTO public.deployment_builds (deployment_id, commit_sha, status, trigger_type, created_at)
		VALUES ($1, $2, 'pending', 'push', NOW())
		RETURNING id, deployment_id, commit_sha, status, trigger_type, created_at
	`
	err = h.db.QueryRowxContext(c.Request.Context(), insertBuild, deployment.ID, payload.After).StructScan(&newBuild)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to queue build: " + err.Error()})
		return
	}

	if pool := worker.GetPool(); pool != nil {
		_ = pool.Enqueue(worker.Job{
			DeploymentID: deployment.ID,
			BuildID:      newBuild.ID,
			RepoURL:      deployment.RepoURL,
			Branch:       deployment.Branch,
			CommitSHA:    payload.After,
			TriggerType:  "push",
		})
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message":  "Build queued from push webhook",
		"build_id": newBuild.ID,
		"commit":   payload.After,
	})
}
