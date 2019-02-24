package models

import "time"

// Thread describes a thread of mails
type Thread struct {
	ID         string
	Authors    string
	NewestDate time.Time
	OldestDate time.Time
	Subject    string
	Tags       []string
	Messages   []string
}
