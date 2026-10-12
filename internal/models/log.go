package models

import "time"

type DeploymentLog struct {
	ID           int64     `json:"id" db:"id"`
	DeploymentID int       `json:"deployment_id" db:"deployment_id"`
	LogType      string    `json:"log_type" db:"log_type"`
	Stream       *string   `json:"stream,omitempty" db:"stream"`
	Message      string    `json:"message" db:"message"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}
