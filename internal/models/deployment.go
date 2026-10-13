package models

import "time"

type DeploymentStatus string

const (
	StatusPending    DeploymentStatus = "pending"
	StatusCloning    DeploymentStatus = "cloning"
	StatusCloned     DeploymentStatus = "cloned"
	StatusBuilding   DeploymentStatus = "building"
	StatusStarting   DeploymentStatus = "starting"
	StatusRunning    DeploymentStatus = "running"
	StatusUnhealthy  DeploymentStatus = "unhealthy"
	StatusFailed     DeploymentStatus = "failed"
	StatusRolledBack DeploymentStatus = "rolled_back"
)

type Deployment struct {
	ID               int              `json:"id" db:"id"`
	UserID           *int             `json:"user_id,omitempty" db:"user_id"`
	RepoURL          string           `json:"repo_url" db:"repo_url"`
	Branch           string           `json:"branch" db:"branch"`
	AutoDeploy       bool             `json:"auto_deploy" db:"auto_deploy"`
	WebhookID        *string          `json:"webhook_id,omitempty" db:"webhook_id"`
	WebhookSecretEnc *string          `json:"-" db:"webhook_secret_enc"`
	CurrentBuildID   *int             `json:"current_build_id,omitempty" db:"current_build_id"`
	URL              *string          `json:"url,omitempty" db:"url"`
	Status           DeploymentStatus `json:"status" db:"status"`
	Port             *int             `json:"port,omitempty" db:"port"`
	ContainerID      *string          `json:"container_id,omitempty" db:"container_id"`
	DockerStatus     *string          `json:"docker_status,omitempty" db:"-"`
	ImageTag         *string          `json:"image_tag,omitempty" db:"image_tag"`
	ErrorMessage     *string          `json:"error_message,omitempty" db:"error_message"`
	BuildNumber      *int             `json:"build_number,omitempty" db:"build_number"`
	CommitSHA        *string          `json:"commit_sha,omitempty" db:"commit_sha"`
	CreatedAt        time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at" db:"updated_at"`
}

type CreateDeploymentRequest struct {
	RepoURL    string            `json:"repo_url" binding:"required"`
	Branch     string            `json:"branch"`
	AutoDeploy *bool             `json:"auto_deploy"`
	EnvVars    map[string]string `json:"env_vars"`
}

type UpdateDeploymentConfigRequest struct {
	Branch     *string           `json:"branch"`
	AutoDeploy *bool             `json:"auto_deploy"`
	EnvVars    map[string]string `json:"env_vars"`
}

type RollbackRequest struct {
	TargetBuildID int `json:"target_build_id" binding:"required"`
}
