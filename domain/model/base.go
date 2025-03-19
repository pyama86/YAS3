package model

import "time"

type Base struct {
	ID        int       `json:"id" dynamo:"id,hash"`
	CreatedAt time.Time `json:"created_at" dynamo:"created_at,range"`
}
