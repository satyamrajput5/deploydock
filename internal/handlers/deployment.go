package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/satyamrajput5/deploydock/internal/builder"
	"github.com/satyamrajput5/deploydock/internal/crypto"
	"github.com/satyamrajput5/deploydock/internal/models"
	"github.com/satyamrajput5/deploydock/internal/worker"
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

	branch := "main"
	if req.Branch != "" {
		branch = req.Branch
	}

	autoDeploy := true
	if req.AutoDeploy != nil {
		autoDeploy = *req.AutoDeploy
	}

	userID := getUserID(c)

	rawSecret := crypto.GenerateRandomHex(16)
	encSecret, _ := crypto.Encrypt(rawSecret)
	webhookID := crypto.GenerateRandomHex(8)

	var deployment models.Deployment
	insertDeployment := `
		INSERT INTO public.deployments (user_id, repo_url, branch, auto_deploy, webhook_id, webhook_secret_enc, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending', NOW(), NOW())
		RETURNING id, user_id, repo_url, branch, auto_deploy, webhook_id, current_build_id, url, status, created_at, updated_at
	`
	err := h.db.QueryRowxContext(c.Request.Context(), insertDeployment, userID, req.RepoURL, branch, autoDeploy, webhookID, encSecret).StructScan(&deployment)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create deployment record: " + err.Error()})
		return
	}

	if len(req.EnvVarItems) > 0 {
		for _, item := range req.EnvVarItems {
			if item.Key == "" {
				continue
			}
			encVal, err := crypto.Encrypt(item.Value)
			if err == nil {
				_, _ = h.db.ExecContext(c.Request.Context(), `
					INSERT INTO public.deployment_env_vars (deployment_id, key, encrypted_value, is_secret, created_at, updated_at)
					VALUES ($1, $2, $3, $4, NOW(), NOW())
					ON CONFLICT (deployment_id, key) DO UPDATE SET encrypted_value = EXCLUDED.encrypted_value, is_secret = EXCLUDED.is_secret, updated_at = NOW()
				`, deployment.ID, item.Key, encVal, item.IsSecret)
			}
		}
	} else if len(req.EnvVars) > 0 {
		for k, v := range req.EnvVars {
			encVal, err := crypto.Encrypt(v)
			if err == nil {
				_, _ = h.db.ExecContext(c.Request.Context(), `
					INSERT INTO public.deployment_env_vars (deployment_id, key, encrypted_value, is_secret, created_at, updated_at)
					VALUES ($1, $2, $3, false, NOW(), NOW())
					ON CONFLICT (deployment_id, key) DO UPDATE SET encrypted_value = EXCLUDED.encrypted_value, updated_at = NOW()
				`, deployment.ID, k, encVal)
			}
		}
	}

	var build models.DeploymentBuild
	insertBuild := `
		INSERT INTO public.deployment_builds (deployment_id, status, trigger_type, created_at)
		VALUES ($1, 'pending', 'initial', NOW())
		RETURNING id, deployment_id, commit_sha, status, trigger_type, created_at
	`
	err = h.db.QueryRowxContext(c.Request.Context(), insertBuild, deployment.ID).StructScan(&build)
	if err == nil {
		_, _ = h.db.ExecContext(c.Request.Context(), `UPDATE public.deployments SET current_build_id = $1 WHERE id = $2`, build.ID, deployment.ID)
		deployment.CurrentBuildID = &build.ID

		if pool := worker.GetPool(); pool != nil {
			_ = pool.Enqueue(worker.Job{
				DeploymentID: deployment.ID,
				BuildID:      build.ID,
				RepoURL:      deployment.RepoURL,
				Branch:       deployment.Branch,
				TriggerType:  "initial",
			})
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"deployment":     deployment,
		"initial_build":  build,
		"webhook_secret": rawSecret,
	})
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
		SELECT 
			d.id, d.user_id, d.repo_url, d.branch, d.auto_deploy, d.webhook_id, d.current_build_id, d.url,
			COALESCE(b.status, d.status) AS status,
			COALESCE(b.port, d.port) AS port,
			COALESCE(b.container_id, d.container_id) AS container_id,
			COALESCE(b.image_tag, d.image_tag) AS image_tag,
			COALESCE(b.error_message, d.error_message) AS error_message,
			b.commit_sha,
			d.created_at, d.updated_at
		FROM public.deployments d
		LEFT JOIN public.deployment_builds b ON d.current_build_id = b.id
		ORDER BY d.created_at DESC
		LIMIT $1 OFFSET $2
	`
	err := h.db.SelectContext(c.Request.Context(), &deployments, query, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query deployments: " + err.Error()})
		return
	}

	liveMap, _ := builder.GetAllContainerStates(c.Request.Context())
	if liveMap != nil {
		for i := range deployments {
			d := &deployments[i]
			origStatus := d.Status
			if d.ContainerID != nil && *d.ContainerID != "" {
				if live, ok := liveMap[*d.ContainerID]; ok {
					d.DockerStatus = &live.Status
					switch live.State {
					case "running":
						d.Status = models.StatusRunning
					case "restarting":
						d.Status = models.DeploymentStatus("restarting")
					case "exited":
						d.Status = models.DeploymentStatus("stopped")
					case "dead":
						d.Status = models.StatusFailed
					case "paused":
						d.Status = models.DeploymentStatus("paused")
					}
				} else {
					notRunning := "container not found"
					d.DockerStatus = &notRunning
					if d.Status == models.StatusRunning {
						d.Status = models.DeploymentStatus("stopped")
					}
				}

				if d.Status != origStatus {
					_, _ = h.db.ExecContext(c.Request.Context(), `UPDATE public.deployments SET status = $1, updated_at = NOW() WHERE id = $2`, d.Status, d.ID)
					if d.CurrentBuildID != nil {
						_, _ = h.db.ExecContext(c.Request.Context(), `UPDATE public.deployment_builds SET status = $1 WHERE id = $2`, d.Status, *d.CurrentBuildID)
					}
				}
			}
		}
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
		SELECT 
			d.id, d.user_id, d.repo_url, d.branch, d.auto_deploy, d.webhook_id, d.current_build_id, d.url,
			COALESCE(b.status, d.status) AS status,
			COALESCE(b.port, d.port) AS port,
			COALESCE(b.container_id, d.container_id) AS container_id,
			COALESCE(b.image_tag, d.image_tag) AS image_tag,
			COALESCE(b.error_message, d.error_message) AS error_message,
			b.commit_sha,
			d.created_at, d.updated_at
		FROM public.deployments d
		LEFT JOIN public.deployment_builds b ON d.current_build_id = b.id
		WHERE d.id = $1
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

	liveMap, _ := builder.GetAllContainerStates(c.Request.Context())
	if liveMap != nil && deployment.ContainerID != nil && *deployment.ContainerID != "" {
		origStatus := deployment.Status
		if live, ok := liveMap[*deployment.ContainerID]; ok {
			deployment.DockerStatus = &live.Status
			switch live.State {
			case "running":
				deployment.Status = models.StatusRunning
			case "restarting":
				deployment.Status = models.DeploymentStatus("restarting")
			case "exited":
				deployment.Status = models.DeploymentStatus("stopped")
			case "dead":
				deployment.Status = models.StatusFailed
			case "paused":
				deployment.Status = models.DeploymentStatus("paused")
			}
		} else {
			notRunning := "container not found"
			deployment.DockerStatus = &notRunning
			if deployment.Status == models.StatusRunning {
				deployment.Status = models.DeploymentStatus("stopped")
			}
		}

		if deployment.Status != origStatus {
			_, _ = h.db.ExecContext(c.Request.Context(), `UPDATE public.deployments SET status = $1, updated_at = NOW() WHERE id = $2`, deployment.Status, deployment.ID)
			if deployment.CurrentBuildID != nil {
				_, _ = h.db.ExecContext(c.Request.Context(), `UPDATE public.deployment_builds SET status = $1 WHERE id = $2`, deployment.Status, *deployment.CurrentBuildID)
			}
		}
	}

	c.JSON(http.StatusOK, deployment)
}

func (h *DeploymentHandler) UpdateConfig(c *gin.Context) {
	deploymentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid deployment ID format"})
		return
	}

	var req models.UpdateDeploymentConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	if req.Branch != nil && *req.Branch != "" {
		_, err := h.db.ExecContext(c.Request.Context(), `UPDATE public.deployments SET branch = $1, updated_at = NOW() WHERE id = $2`, *req.Branch, deploymentID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update branch: " + err.Error()})
			return
		}
	}

	if req.AutoDeploy != nil {
		_, err := h.db.ExecContext(c.Request.Context(), `UPDATE public.deployments SET auto_deploy = $1, updated_at = NOW() WHERE id = $2`, *req.AutoDeploy, deploymentID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update auto_deploy: " + err.Error()})
			return
		}
	}

	if len(req.EnvVars) > 0 {
		for k, v := range req.EnvVars {
			encVal, err := crypto.Encrypt(v)
			if err == nil {
				_, _ = h.db.ExecContext(c.Request.Context(), `
					INSERT INTO public.deployment_env_vars (deployment_id, key, encrypted_value, created_at, updated_at)
					VALUES ($1, $2, $3, NOW(), NOW())
					ON CONFLICT (deployment_id, key) DO UPDATE SET encrypted_value = EXCLUDED.encrypted_value, updated_at = NOW()
				`, deploymentID, k, encVal)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Configuration updated successfully"})
}

func (h *DeploymentHandler) Redeploy(c *gin.Context) {
	deploymentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid deployment ID format"})
		return
	}

	var deployment models.Deployment
	if err := h.db.GetContext(c.Request.Context(), &deployment, `SELECT id, repo_url, branch FROM public.deployments WHERE id = $1`, deploymentID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	var newBuild models.DeploymentBuild
	query := `
		INSERT INTO public.deployment_builds (deployment_id, status, trigger_type, created_at)
		VALUES ($1, 'pending', 'manual', NOW())
		RETURNING id, deployment_id, commit_sha, status, trigger_type, created_at
	`
	if err := h.db.QueryRowxContext(c.Request.Context(), query, deploymentID).StructScan(&newBuild); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to queue redeploy build: " + err.Error()})
		return
	}

	if pool := worker.GetPool(); pool != nil {
		_ = pool.Enqueue(worker.Job{
			DeploymentID: deployment.ID,
			BuildID:      newBuild.ID,
			RepoURL:      deployment.RepoURL,
			Branch:       deployment.Branch,
			TriggerType:  "manual",
		})
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Redeploy triggered",
		"build":   newBuild,
	})
}

func (h *DeploymentHandler) ListBuilds(c *gin.Context) {
	deploymentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid deployment ID format"})
		return
	}

	builds := make([]models.DeploymentBuild, 0)
	query := `
		SELECT id, deployment_id, commit_sha, status, image_tag, container_id, port, error_message, trigger_type, started_at, finished_at, created_at
		FROM public.deployment_builds
		WHERE deployment_id = $1
		ORDER BY created_at DESC
	`
	if err := h.db.SelectContext(c.Request.Context(), &builds, query, deploymentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list builds: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deployment_id": deploymentID,
		"builds":        builds,
		"count":         len(builds),
	})
}

func (h *DeploymentHandler) GetBuild(c *gin.Context) {
	deploymentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid deployment ID"})
		return
	}

	buildID, err := strconv.Atoi(c.Param("buildId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid build ID"})
		return
	}

	var build models.DeploymentBuild
	query := `
		SELECT id, deployment_id, commit_sha, status, image_tag, container_id, port, error_message, trigger_type, started_at, finished_at, created_at
		FROM public.deployment_builds
		WHERE id = $1 AND deployment_id = $2
	`
	if err := h.db.GetContext(c.Request.Context(), &build, query, buildID, deploymentID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Build revision not found"})
		return
	}

	c.JSON(http.StatusOK, build)
}

func (h *DeploymentHandler) Rollback(c *gin.Context) {
	deploymentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid deployment ID format"})
		return
	}

	var req models.RollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	var targetBuild models.DeploymentBuild
	query := `SELECT id, deployment_id, status FROM public.deployment_builds WHERE id = $1 AND deployment_id = $2`
	if err := h.db.GetContext(c.Request.Context(), &targetBuild, query, req.TargetBuildID, deploymentID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Target build not found for this deployment"})
		return
	}

	_, err = h.db.ExecContext(c.Request.Context(), `
		UPDATE public.deployments
		SET current_build_id = $1, status = 'rolled_back', updated_at = NOW()
		WHERE id = $2
	`, targetBuild.ID, deploymentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to switch traffic to target build: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Traffic successfully switched to target build",
		"target_build_id": targetBuild.ID,
	})
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

	buildIDQuery := c.Query("build_id")
	logs := make([]models.DeploymentLog, 0)

	var query string
	var args []interface{}

	if buildIDQuery != "" {
		if bID, err := strconv.Atoi(buildIDQuery); err == nil {
			query = `
				SELECT id, deployment_id, build_id, log_type, stream, message, created_at
				FROM public.deployment_logs
				WHERE deployment_id = $1 AND build_id = $2
				ORDER BY created_at ASC
			`
			args = []interface{}{deploymentID, bID}
		}
	}

	if query == "" {
		query = `
			SELECT id, deployment_id, build_id, log_type, stream, message, created_at
			FROM public.deployment_logs
			WHERE deployment_id = $1
			ORDER BY created_at ASC
		`
		args = []interface{}{deploymentID}
	}

	if err := h.db.SelectContext(c.Request.Context(), &logs, query, args...); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve logs: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deployment_id": deploymentID,
		"logs":          logs,
	})
}

func (h *DeploymentHandler) InspectRepo(c *gin.Context) {
	var req models.InspectRepoRequest
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

	resp, err := builder.InspectRepo(c.Request.Context(), req.RepoURL)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to inspect repository: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

