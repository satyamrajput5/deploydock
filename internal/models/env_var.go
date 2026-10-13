package models

import "time"

type DeploymentEnvVar struct {
	ID             int       `json:"id" db:"id"`
	DeploymentID   int       `json:"deployment_id" db:"deployment_id"`
	Key            string    `json:"key" db:"key"`
	EncryptedValue string    `json:"-" db:"encrypted_value"`
	IsSecret       bool      `json:"is_secret" db:"is_secret"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}
