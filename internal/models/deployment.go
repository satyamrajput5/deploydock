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
	ID           int              `json:"id" db:"id"`
	UserID       *int             `json:"user_id,omitempty" db:"user_id"`
	RepoURL      string           `json:"repo_url" db:"repo_url"`
	Status       DeploymentStatus `json:"status" db:"status"`
	Port         *int             `json:"port,omitempty" db:"port"`
	ContainerID  *string          `json:"container_id,omitempty" db:"container_id"`
	ImageTag     *string          `json:"image_tag,omitempty" db:"image_tag"`
	URL          *string          `json:"url,omitempty" db:"url"`
	ErrorMessage *string          `json:"error_message,omitempty" db:"error_message"`
	BuildNumber  *int             `json:"build_number,omitempty" db:"build_number"`
	CreatedAt    time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at" db:"updated_at"`
}

type CreateDeploymentRequest struct {
	RepoURL string `json:"repo_url" binding:"required"`
}
