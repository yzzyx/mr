package config

import "github.com/yzzyx/mr/imap"

type Config struct {
	Maildir   string
	Mailboxes map[string]imap.Mailbox
}
