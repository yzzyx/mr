package models

import "github.com/yzzyx/mr/notmuch"

var (
	notmuchDB *notmuch.Database
)

// Setup initializes the global notmuch-database
func Setup(db *notmuch.Database) error {
	notmuchDB = db
	return nil
}
