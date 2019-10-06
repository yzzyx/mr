package models

import "github.com/yzzyx/mr/notmuch"

var (
	notmuchDB *notmuch.Database
)

func Setup(db *notmuch.Database) error {
	notmuchDB = db
	return nil
}
