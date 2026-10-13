package worker

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/satyamrajput5/deploydock/internal/builder"
	"github.com/satyamrajput5/deploydock/internal/crypto"
)

type Job struct {
	DeploymentID int
	BuildID      int
	RepoURL      string
	Branch       string
	CommitSHA    string
	TriggerType  string
}

type WorkerPool struct {
	db    *sqlx.DB
	queue chan Job
}

var globalPool *WorkerPool

func NewWorkerPool(db *sqlx.DB, bufferSize int) *WorkerPool {
	pool := &WorkerPool{
		db:    db,
		queue: make(chan Job, bufferSize),
	}
	globalPool = pool
	return pool
}

func GetPool() *WorkerPool {
	return globalPool
}

func (p *WorkerPool) Enqueue(job Job) error {
	select {
	case p.queue <- job:
		return nil
	default:
		return fmt.Errorf("build queue is full")
	}
}

func (p *WorkerPool) Start(ctx context.Context, numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			for {
				select {
				case <-ctx.Done():
					return
				case job := <-p.queue:
					p.processJob(job)
				}
			}
		}(i)
	}
}

func (p *WorkerPool) appendLog(deploymentID, buildID int, stream, message string) {
	_, _ = p.db.Exec(`
		INSERT INTO public.deployment_logs (deployment_id, build_id, log_type, stream, message, created_at)
		VALUES ($1, $2, 'build', $3, $4, NOW())
	`, deploymentID, buildID, stream, message)
}

func (p *WorkerPool) processJob(job Job) {
	now := time.Now()
	_, _ = p.db.Exec(`UPDATE public.deployment_builds SET status = 'building', started_at = $1 WHERE id = $2`, now, job.BuildID)
	_, _ = p.db.Exec(`UPDATE public.deployments SET status = 'building', updated_at = NOW() WHERE id = $1`, job.DeploymentID)

	p.appendLog(job.DeploymentID, job.BuildID, "stdout", fmt.Sprintf("Build started (Trigger: %s)", job.TriggerType))

	destDir := filepath.Join(".", "repos", fmt.Sprintf("%d", job.DeploymentID), fmt.Sprintf("%d", job.BuildID))

	var cloneBuf bytes.Buffer
	p.appendLog(job.DeploymentID, job.BuildID, "stdout", fmt.Sprintf("Cloning %s (%s)...", job.RepoURL, job.Branch))
	err := builder.Clone(context.Background(), job.RepoURL, job.Branch, job.CommitSHA, destDir, &cloneBuf)
	if cloneBuf.Len() > 0 {
		p.appendLog(job.DeploymentID, job.BuildID, "stdout", cloneBuf.String())
	}
	if err != nil {
		p.failBuild(job, fmt.Sprintf("Git clone failed: %v", err))
		return
	}

	detectResult, err := builder.FindOrGenerateDockerfile(destDir)
	if err != nil {
		p.failBuild(job, fmt.Sprintf("Dockerfile preparation failed: %v", err))
		return
	}

	if detectResult.Generated {
		p.appendLog(job.DeploymentID, job.BuildID, "stdout", fmt.Sprintf("No pre-existing Dockerfile found. Auto-detected runtime [%s] and generated tailored Dockerfile.", detectResult.TechStack))
	} else {
		p.appendLog(job.DeploymentID, job.BuildID, "stdout", fmt.Sprintf("Found pre-existing Dockerfile [%s]. Using custom configuration.", filepath.Base(detectResult.DockerfilePath)))
	}

	imageTag := fmt.Sprintf("deploydock-app-%d:%d", job.DeploymentID, job.BuildID)
	var buildBuf bytes.Buffer
	p.appendLog(job.DeploymentID, job.BuildID, "stdout", fmt.Sprintf("Building Docker image %s...", imageTag))
	err = builder.Build(context.Background(), imageTag, destDir, detectResult.DockerfilePath, &buildBuf)
	if buildBuf.Len() > 0 {
		p.appendLog(job.DeploymentID, job.BuildID, "stdout", buildBuf.String())
	}
	if err != nil {
		p.failBuild(job, fmt.Sprintf("Docker build failed: %v", err))
		return
	}

	allocatedPort, err := builder.AllocatePort(p.db)
	if err != nil {
		p.failBuild(job, fmt.Sprintf("Port allocation failed: %v", err))
		return
	}

	envMap := make(map[string]string)
	type envRow struct {
		Key            string `db:"key"`
		EncryptedValue string `db:"encrypted_value"`
	}
	var envRows []envRow
	_ = p.db.Select(&envRows, `SELECT key, encrypted_value FROM public.deployment_env_vars WHERE deployment_id = $1`, job.DeploymentID)
	for _, r := range envRows {
		val, decErr := crypto.Decrypt(r.EncryptedValue)
		if decErr == nil {
			envMap[r.Key] = val
		}
	}

	p.appendLog(job.DeploymentID, job.BuildID, "stdout", fmt.Sprintf("Starting container on port :%d...", allocatedPort))
	containerID, err := builder.Run(context.Background(), job.DeploymentID, job.BuildID, imageTag, allocatedPort, envMap)
	if err != nil {
		p.failBuild(job, fmt.Sprintf("Container run failed: %v", err))
		return
	}

	time.Sleep(2 * time.Second)
	ci, inspErr := builder.Inspect(context.Background(), containerID)
	if inspErr == nil && ci != nil {
		if ci.Restarting || (!ci.Running && ci.Status != "running") {
			containerLogs := builder.GetLogs(context.Background(), containerID, 40)
			_ = builder.Stop(context.Background(), containerID)
			failMsg := fmt.Sprintf("Container failed to stay alive (status: %s, exit code: %d). Ensure your application starts a persistent HTTP server listening on port 8080.", ci.Status, ci.ExitCode)
			if containerLogs != "" {
				failMsg = fmt.Sprintf("%s\n\nContainer output:\n%s", failMsg, containerLogs)
			}
			p.failBuild(job, failMsg)
			return
		}
	}

	finishedAt := time.Now()
	liveURL := fmt.Sprintf("http://localhost:%d", allocatedPort)

	_, _ = p.db.Exec(`
		UPDATE public.deployment_builds 
		SET status = 'running', container_id = $1, port = $2, image_tag = $3, finished_at = $4
		WHERE id = $5
	`, containerID, allocatedPort, imageTag, finishedAt, job.BuildID)

	var oldBuildID *int
	_ = p.db.Get(&oldBuildID, `SELECT current_build_id FROM public.deployments WHERE id = $1`, job.DeploymentID)

	_, _ = p.db.Exec(`
		UPDATE public.deployments
		SET current_build_id = $1, status = 'running', port = $2, url = $3, container_id = $4, image_tag = $5, updated_at = NOW()
		WHERE id = $6
	`, job.BuildID, allocatedPort, liveURL, containerID, imageTag, job.DeploymentID)

	if oldBuildID != nil && *oldBuildID != job.BuildID {
		var oldContainerID *string
		_ = p.db.Get(&oldContainerID, `SELECT container_id FROM public.deployment_builds WHERE id = $1`, *oldBuildID)
		if oldContainerID != nil && *oldContainerID != "" {
			_ = builder.Stop(context.Background(), *oldContainerID)
			_, _ = p.db.Exec(`UPDATE public.deployment_builds SET status = 'rolled_back' WHERE id = $1`, *oldBuildID)
		}
	}

	p.appendLog(job.DeploymentID, job.BuildID, "stdout", fmt.Sprintf("Deployment successfully live at %s", liveURL))
	log.Printf("SUCCESS: Deployment %d build %d is now live at %s", job.DeploymentID, job.BuildID, liveURL)
}

func (p *WorkerPool) failBuild(job Job, errorMsg string) {
	log.Printf("ERROR: Deployment %d build %d failed: %s", job.DeploymentID, job.BuildID, errorMsg)
	p.appendLog(job.DeploymentID, job.BuildID, "stderr", errorMsg)

	now := time.Now()
	_, _ = p.db.Exec(`
		UPDATE public.deployment_builds
		SET status = 'failed', error_message = $1, finished_at = $2
		WHERE id = $3
	`, errorMsg, now, job.BuildID)

	var hasHealthyLive bool
	_ = p.db.Get(&hasHealthyLive, `
		SELECT EXISTS(
			SELECT 1 FROM public.deployment_builds 
			WHERE deployment_id = $1 AND id != $2 AND status = 'running'
		)
	`, job.DeploymentID, job.BuildID)

	if !hasHealthyLive {
		_, _ = p.db.Exec(`
			UPDATE public.deployments
			SET status = 'failed', error_message = $1, updated_at = NOW()
			WHERE id = $2
		`, errorMsg, job.DeploymentID)
	}
}
