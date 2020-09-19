package models

import "time"

// Message describes a single message
type Message struct {
	ID       string
	Filename string
	Date     time.Time
}
