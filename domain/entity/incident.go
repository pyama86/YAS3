package entity

import "time"

// LinkedChannel は紐づけられたチャンネル/スレッド情報
type LinkedChannel struct {
	ChannelID string `json:"channel_id"`
	ThreadTS  string `json:"thread_ts,omitempty"` // スレッドの場合のみ設定
}

type Incident struct {
	ChannelID              string          `json:"channel_id" dynamo:"channel_id,hash"`
	Description            string          `json:"description" dynamo:"description"`
	Urgency                string          `json:"urgency_level" dynamo:"urgency_level"`
	Level                  int             `json:"level" dynamo:"level"`
	ServiceID              int             `json:"service_id" dynamo:"service_id"`
	HandlerUserID          string          `json:"handler_user_id" dynamo:"handler_user_id"`
	CreatedUserID          string          `json:"created_user_id" dynamo:"created_user_id"`
	RecoveredUserID        string          `json:"recovered_user_id" dynamo:"recovered_user_id"`
	DisableTimer           bool            `json:"disable_timer" dynamo:"disable_timer"`
	PostMortemURL          string          `json:"post_mortem_url" dynamo:"post_mortem_url"`
	ReopenedAt             time.Time       `json:"reopened_at" dynamo:"reopened_at"`
	ReopenedUserID         string          `json:"reopened_user_id" dynamo:"reopened_user_id"`
	StartedAt              time.Time       `json:"started_at" dynamo:"started_at"`
	RecoveredAt            time.Time       `json:"recovered_at" dynamo:"recovered_at"`
	ClosedAt               time.Time       `json:"closed_at" dynamo:"closed_at"`
	LastSummary            string          `json:"last_summary" dynamo:"last_summary"`
	LastSummaryAt          time.Time       `json:"last_summary_at" dynamo:"last_summary_at"`
	LastProcessedMessageTS string          `json:"last_processed_message_ts" dynamo:"last_processed_message_ts"`
	LinkedChannels         []LinkedChannel `json:"linked_channels" dynamo:"linked_channels"`
}
