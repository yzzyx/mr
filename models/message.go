package models

import "time"

type Message struct {
	ID       string
	Filename string
	Date     time.Time
}
