package models

import (
	"time"

	"github.com/yzzyx/mr/notmuch"
)

// Thread describes a thread of mails
type Thread struct {
	ID         string
	Authors    string
	NewestDate time.Time
	OldestDate time.Time
	Subject    string
	Tags       []string
	Messages   []Message
}

func (t Thread) SaveTags() {
	for _, msg := range t.Messages {
		m, status := notmuchDB.FindMessage(msg.ID)
		if status != notmuch.STATUS_SUCCESS {
			continue
		}

		// Check if there's any tags we need to remove
		tagIterator := m.GetTags()
		currentTags := map[string]struct{}{}
		for tagIterator.Valid() {
			currentTags[tagIterator.Get()] = struct{}{}
			tagIterator.MoveToNext()
		}

		for _, nt := range t.Tags {
			if _, ok := currentTags[nt]; ok {
				delete(currentTags, nt)
			}
			m.AddTag(nt)
		}

		for tag := range currentTags {
			m.RemoveTag(tag)
		}
	}
}

// HasTag returns true if thread has a specific tag set
func (t *Thread) HasTag(tag string) bool {
	for _, t := range t.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// FilterTags returns a filtered list of tags (hides unread, inbox)
func (t *Thread) FilterTags() []string {
	result := make([]string, 0, len(t.Tags))
	excludeTags := map[string]struct{}{
		"unread": struct{}{},
		"inbox":  struct{}{},
	}
	for _, t := range t.Tags {
		if _, skip := excludeTags[t]; skip {
			continue
		}
		result = append(result, t)
	}

	return result
}
