package models

type Replica struct {
	ID           int     `json:"id" db:"id"`
	DeploymentID int     `json:"deployment_id" db:"deployment_id"`
	ContainerID  *string `json:"container_id,omitempty" db:"container_id"`
	Port         *int    `json:"port,omitempty" db:"port"`
	Status       string  `json:"status" db:"status"`
}
