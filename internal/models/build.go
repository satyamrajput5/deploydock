package models

import "time"

type DeploymentBuild struct {
	ID           int              `json:"id" db:"id"`
	DeploymentID int              `json:"deployment_id" db:"deployment_id"`
	CommitSHA    *string          `json:"commit_sha,omitempty" db:"commit_sha"`
	Status       DeploymentStatus `json:"status" db:"status"`
	ImageTag     *string          `json:"image_tag,omitempty" db:"image_tag"`
	ContainerID  *string          `json:"container_id,omitempty" db:"container_id"`
	Port         *int             `json:"port,omitempty" db:"port"`
	ErrorMessage *string          `json:"error_message,omitempty" db:"error_message"`
	TriggerType  string           `json:"trigger_type" db:"trigger_type"`
	StartedAt    *time.Time       `json:"started_at,omitempty" db:"started_at"`
	FinishedAt   *time.Time       `json:"finished_at,omitempty" db:"finished_at"`
	CreatedAt    time.Time        `json:"created_at" db:"created_at"`
}
