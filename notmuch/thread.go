package notmuch

/*
#cgo LDFLAGS: -lnotmuch

#include <stdlib.h>
#include <string.h>
#include <time.h>
#include "notmuch.h"
*/
import "C"

import "time"

/**
 * Get the thread ID of 'thread'.
 *
 * The returned string belongs to 'thread' and as such, should not be
 * modified by the caller and will only be valid for as long as the
 * thread is valid, (which is until notmuch_thread_destroy or until
 * the query from which it derived is destroyed).
 */
func (self *Thread) GetThreadID() string {
	if self.thread == nil {
		return ""
	}

	cstr := C.notmuch_thread_get_thread_id(self.thread)
	return C.GoString(cstr)
}

/**
 * Get the total number of messages in 'thread'.
 *
 * This count consists of all messages in the database belonging to
 * this thread. Contrast with notmuch_thread_get_matched_messages() .
 */
func (self *Thread) GetTotalMessages() int {
	if self.thread == nil {
		return 0
	}

	cnt := C.notmuch_thread_get_total_messages(self.thread)
	return int(cnt)
}

/**
 * Get a notmuch_messages_t iterator for the top-level messages in
 * 'thread' in oldest-first order.
 *
 * This iterator will not necessarily iterate over all of the messages
 * in the thread. It will only iterate over the messages in the thread
 * which are not replies to other messages in the thread.
 *
 * The returned list will be destroyed when the thread is destroyed.
 */
func (self *Thread) GetToplevelMessages() *Messages {
	msgs := C.notmuch_thread_get_toplevel_messages(self.thread)
	if msgs == nil {
		return nil
	}
	return &Messages{messages: msgs}
}

/**
 * Get a notmuch_thread_t iterator for all messages in 'thread' in
 * oldest-first order.
 *
 * The returned list will be destroyed when the thread is destroyed.
 */
func (self *Thread) GetMessages() *Messages {
	msgs := C.notmuch_thread_get_messages(self.thread)
	if msgs == nil {
		return nil
	}
	return &Messages{messages: msgs}
}

/**
 * Get the number of messages in 'thread' that matched the search.
 *
 * This count includes only the messages in this thread that were
 * matched by the search from which the thread was created and were
 * not excluded by any exclude tags passed in with the query (see
 * notmuch_query_add_tag_exclude). Contrast with
 * notmuch_thread_get_total_messages() .
 */
func (self *Thread) GetMatchedMessages() int {
	if self.thread == nil {
		return 0
	}

	cnt := C.notmuch_thread_get_matched_messages(self.thread)
	return int(cnt)
}

/**
 * Get the authors of 'thread' as a UTF-8 string.
 *
 * The returned string is a comma-separated list of the names of the
 * authors of mail messages in the query results that belong to this
 * thread.
 *
 * The string contains authors of messages matching the query first, then
 * non-matched authors (with the two groups separated by '|'). Within
 * each group, authors are ordered by date.
 *
 * The returned string belongs to 'thread' and as such, should not be
 * modified by the caller and will only be valid for as long as the
 * thread is valid, (which is until notmuch_thread_destroy or until
 * the query from which it derived is destroyed).
 */
func (self *Thread) GetAuthors() string {
	if self.thread == nil {
		return ""
	}

	cstr := C.notmuch_thread_get_authors(self.thread)
	return C.GoString(cstr)
}

/**
 * Get the subject of 'thread' as a UTF-8 string.
 *
 * The subject is taken from the first message (according to the query
 * order---see notmuch_query_set_sort) in the query results that
 * belongs to this thread.
 *
 * The returned string belongs to 'thread' and as such, should not be
 * modified by the caller and will only be valid for as long as the
 * thread is valid, (which is until notmuch_thread_destroy or until
 * the query from which it derived is destroyed).
 */
func (self *Thread) GetSubject() string {
	if self.thread == nil {
		return ""
	}

	cstr := C.notmuch_thread_get_subject(self.thread)
	return C.GoString(cstr)
}

/* Get the date of the oldest message in 'thread' as a time_t value. */
func (self *Thread) GetOldestDate() time.Time {
	if self.thread == nil {
		return time.Time{}
	}

	t := C.notmuch_thread_get_oldest_date(self.thread)
	return time.Unix(int64(t), 0)
}

/* Get the date of the newest message in 'thread' as a time_t value. */
func (self *Thread) GetNewestDate() time.Time {
	if self.thread == nil {
		return time.Time{}
	}

	t := C.notmuch_thread_get_newest_date(self.thread)
	return time.Unix(int64(t), 0)
}

/**
 * Get the tags for 'thread', returning a notmuch_tags_t object which
 * can be used to iterate over all tags.
 *
 * Note: In the Notmuch database, tags are stored on individual
 * messages, not on threads. So the tags returned here will be all
 * tags of the messages which matched the search and which belong to
 * this thread.
 *
 * The tags object is owned by the thread and as such, will only be
 * valid for as long as the thread is valid, (for example, until
 * notmuch_thread_destroy or until the query from which it derived is
 * destroyed).
 *
 * Typical usage might be:
 *
 *     notmuch_thread_t *thread;
 *     notmuch_tags_t *tags;
 *     const char *tag;
 *
 *     thread = notmuch_threads_get (threads);
 *
 *     for (tags = notmuch_thread_get_tags (thread);
 *          notmuch_tags_valid (tags);
 *          notmuch_tags_move_to_next (tags))
 *     {
 *         tag = notmuch_tags_get (tags);
 *         ....
 *     }
 *
 *     notmuch_thread_destroy (thread);
 *
 * Note that there's no explicit destructor needed for the
 * notmuch_tags_t object. (For consistency, we do provide a
 * notmuch_tags_destroy function, but there's no good reason to call
 * it if the message is about to be destroyed).
 */
func (self *Thread) GetTags() *Tags {
	if self.thread == nil {
		return nil
	}
	tags := C.notmuch_thread_get_tags(self.thread)
	if tags == nil {
		return nil
	}
	return &Tags{tags: tags}
}

/* Destroy a notmuch_thread_t object. */
func (self *Thread) Destroy() {
	if self.thread == nil {
		return
	}
	C.notmuch_thread_destroy(self.thread)
}
