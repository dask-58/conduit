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

type DeliveryLog struct {
	ID           string         `json:"id" db:"id"`
	EventID      string         `json:"event_id" db:"event_id"`
	EndpointID   string         `json:"endpoint_id" db:"endpoint_id"`
	Status       DeliveryStatus `json:"status" db:"status"`
	Attempt      int            `json:"attempt" db:"attempt"`
	NextRetryAt  *time.Time     `json:"next_retry_at,omitempty" db:"next_retry_at"`
	ResponseCode *int           `json:"response_code,omitempty" db:"response_code"`
	DeliveredAt  *time.Time     `json:"delivered_at,omitempty" db:"delivered_at"`
	CreatedAt    time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at" db:"updated_at"`
}

type EventSummary struct {
	Event
	TotalDeliveries int    `json:"total_deliveries"`
	Pending         int    `json:"pending"`
	Success         int    `json:"success"`
	Failed          int    `json:"failed"`
	Exhausted       int    `json:"exhausted"`
	SummaryStatus   string `json:"summary_status"`
}
