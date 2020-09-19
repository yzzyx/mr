package config

import "github.com/yzzyx/mr/imap"

// Config describes the available configuration layout
type Config struct {
	Maildir   string
	Mailboxes map[string]imap.Mailbox
}
