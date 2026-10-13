package models

import "time"

type WebhookDelivery struct {
	ID               int64     `json:"id" db:"id"`
	DeliveryID       string    `json:"delivery_id" db:"delivery_id"`
	DeploymentID     int       `json:"deployment_id" db:"deployment_id"`
	EventType        string    `json:"event_type" db:"event_type"`
	CommitSHA        *string   `json:"commit_sha,omitempty" db:"commit_sha"`
	ProcessingStatus string    `json:"processing_status" db:"processing_status"`
	ReceivedAt       time.Time `json:"received_at" db:"received_at"`
}
