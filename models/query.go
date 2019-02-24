package models

// Query describes a query for a list of mailthreads
type Query struct {
	query string
	rows  []Thread
	count int
}

func NewQuery(query string) *Query {
	return &Query{query: query}
}

func (m *Query) Count() int {
	if m.count == 0 {
		q := notmuchDB.CreateQuery(m.query)
		defer q.Destroy()
		m.count = int(q.CountThreads())
	}

	return m.count
}

func (m *Query) GetLine(lineNumber int) Thread {
	if lineNumber >= len(m.rows) || m.rows == nil {
		m.rows = append(m.rows, m.GetList(len(m.rows), lineNumber+50)...)
	}

	if lineNumber > len(m.rows) {
		return Thread{}
	}
	return m.rows[lineNumber]
}

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
		messageList := []string{}
		for messages.Valid() {
			m := messages.Get()
			messageList = append(messageList, m.GetFileName())
			messages.MoveToNext()
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
