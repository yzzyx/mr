package models

import (
	"time"

	"github.com/yzzyx/mr/notmuch"
)

// Query describes a query for a list of mailthreads
type Query struct {
	query string
	rows  []Thread
	count int
}

// NewQuery creates a new query
func NewQuery(query string) *Query {
	return &Query{query: query}
}

// Count returns the matching query count
func (m *Query) Count() int {
	if m.count == 0 {
		q := notmuchDB.CreateQuery(m.query)
		defer q.Destroy()
		m.count = int(q.CountThreads())
	}

	return m.count
}

// GetLine returns the contents of a specific line
func (m *Query) GetLine(lineNumber int) Thread {
	if lineNumber >= len(m.rows) || m.rows == nil {
		m.rows = append(m.rows, m.GetList(len(m.rows), lineNumber+50)...)
	}

	if lineNumber > len(m.rows) {
		return Thread{}
	}
	return m.rows[lineNumber]
}

// GetList returns the threads available between 'from' and 'to'
func (m *Query) GetList(from, to int) []Thread {
	q := notmuchDB.CreateQuery(m.query)
	defer q.Destroy()

	result := make([]Thread, 0, to-from)

	// FIXME - there must be a way to do this where we don't have to iterate through everything
	threads := q.SearchThreads()
	defer threads.Destroy()
	cnt := -1
	for threads.Valid() {
		cnt++
		if cnt < from {
			threads.MoveToNext()
			continue
		}
		if cnt > to {
			break
		}

		thread := threads.Get()
		tags := thread.GetTags()
		tagList := []string{}
		for tags.Valid() {
			tagList = append(tagList, tags.Get())
			tags.MoveToNext()
		}

		messages := thread.GetMessages()
		messageList := make([]Message, 0, thread.GetTotalMessages())
		for messages.Valid() {
			m := messages.Get()

			message := Message{
				ID:       m.GetMessageId(),
				Filename: m.GetFileName(),
			}

			messageTimestamp, status := m.GetDate()
			if status == notmuch.STATUS_SUCCESS {
				message.Date = time.Unix(messageTimestamp, 0)
			}

			messageList = append(messageList, message)

			messages.MoveToNext()
		}

		// Reverse list of messages
		for left, right := 0, len(messageList)-1; left < right; left, right = left+1, right-1 {
			messageList[left], messageList[right] = messageList[right], messageList[left]
		}

		t := Thread{
			ID:         thread.GetThreadID(),
			Authors:    thread.GetAuthors(),
			NewestDate: thread.GetNewestDate(),
			OldestDate: thread.GetOldestDate(),
			Subject:    thread.GetSubject(),
			Tags:       tagList,
			Messages:   messageList,
		}

		result = append(result, t)
		threads.MoveToNext()
	}
	return result
}
