package entity

import "time"

type IncidentTypeEnum int

const (
	IncidentTypeEnumSecurity IncidentTypeEnum = iota
	IncidentTypeEnumService
	IncidentTypeEnumOther
)

type Incident struct {
	ChannelID       string    `json:"channel_id" dynamo:"channel_id,hash"`
	Description     string    `json:"description" dynamo:"description"`
	Urgency         string    `json:"urgency_level" dynamo:"urgency_level"`
	Level           int       `json:"level" dynamo:"level"`
	ServiceID       int       `json:"service_id" dynamo:"service_id"`
	HandlerUserID   string    `json:"handler_user_id" dynamo:"handler_user_id"`
	CreatedUserID   string    `json:"created_user_id" dynamo:"created_user_id"`
	RecoveredUserID string    `json:"recovered_user_id" dynamo:"recovered_user_id"`
	DisableTimer    bool      `json:"disable_timer" dynamo:"disable_timer"`
	PostMortemURL   string    `json:"post_mortem_url" dynamo:"post_mortem_url"`
	StartedAt       time.Time `json:"started_at" dynamo:"started_at"`
	RecoveredAt     time.Time `json:"recovered_at" dynamo:"recovered_at"`
	ClosedAt        time.Time `json:"closed_at" dynamo:"closed_at"`
}
