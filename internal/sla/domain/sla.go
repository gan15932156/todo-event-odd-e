package domain

import "time"

type SLA struct {
	ID    string    `json:"id"`
	Time  time.Time `json:"time"`
	Event string    `json:"event"`
}
