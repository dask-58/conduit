package model

import (
	"encoding/json"
	"time"
)

type Event struct {
	ID         string          `json:"id" db:"id"`
	Source     string          `json:"source" db:"source"`
	Payload    json.RawMessage `json:"payload" db:"payload"`
	ReceivedAt time.Time       `json:"received_at" db:"received_at"`
}

type Endpoint struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	URL       string    `json:"url" db:"url"`
	Secret    string    `json:"secret,omitempty" db:"secret"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type DeliveryJob struct {
	EventID    string          `json:"event_id"`
	EndpointID string          `json:"endpoint_id"`
	Attempt    int             `json:"attempt"`
	Payload    json.RawMessage `json:"payload"`
}

type DeliveryStatus string

const (
	StatusPending   DeliveryStatus = "pending"
	StatusSuccess   DeliveryStatus = "success"
	StatusFailed    DeliveryStatus = "failed"
	StatusExhausted DeliveryStatus = "exhausted"
)
