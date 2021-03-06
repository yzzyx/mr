package imap

import (
	"crypto/md5"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/yzzyx/mr/notmuch"
)

// Mailbox defines the available options for a IMAP mailbox to pull from
type Mailbox struct {
	Server      string
	Port        int
	Username    string
	Password    string
	UseTLS      bool `yaml:"use_tls"`
	UseStartTLS bool `yaml:"use_starttls"`
	Folders     struct {
		Include []string
		Exclude []string
	}

	FolderTags map[string]string `yaml:"folder_tags"`
}

type mailConfig struct {
	// Keep track of last seen UID for each mailbox
	LastSeenUID map[string]uint32
}

// IndexUpdate is used to signal that a message should be tagged with specific information
type IndexUpdate struct {
	Path      string   // Path to file to be updated
	MessageID string   // MessageID to be updated
	Tags      []string // Tags to add/remove from message (entries prefixed with "-" will be removed)
}

// Handler is responsible for reading from mailboxes and updating the notmuch index
// Note that a single handler can only read from one mailbox
type Handler struct {
	db          *notmuch.Database
	maildirPath string
	mailbox     Mailbox

	cfg mailConfig

	// Used internally to generate maildir files
	seqNumChan <-chan int
	processID  int
	hostname   string
}

// New creates a new Handler
func New(db *notmuch.Database,
	maildirPath string,
	mailbox Mailbox) (*Handler, error) {

	var err error
	h := Handler{}
	h.hostname, err = os.Hostname()
	if err != nil {
		return nil, err
	}

	h.mailbox = mailbox

	// Generate unique sequence numbers
	seqNumChan := make(chan int)
	go func() {
		seqNum := 1
		for {
			seqNumChan <- seqNum
			seqNum++
		}
	}()
	h.seqNumChan = seqNumChan
	h.processID = os.Getpid()
	h.db = db
	h.maildirPath = maildirPath

	h.cfg.LastSeenUID = make(map[string]uint32)
	// Get list of timestamps etc.
	data, err := ioutil.ReadFile(filepath.Join(maildirPath, ".imap-uids"))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		err = json.Unmarshal(data, &h.cfg)
		if err != nil {
			return nil, err
		}
	}
	return &h, nil
}

// Close closes all open handles, flushes channels and saves configuration data
func (h *Handler) Close() error {
	data, err := json.Marshal(h.cfg)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(h.maildirPath, ".imap-uids"), data, 0700)
	if err != nil {
		return err
	}

	return nil
}

// getMessage downloads a message from the server from a mailbox, and stores it in a maildir
func (h *Handler) getMessage(c *client.Client, mailbox string, uid uint32) error {
	// Select INBOX
	_, err := c.Select(mailbox, false)
	if err != nil {
		return err
	}

	// Get the whole message body
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem()}
	messages := make(chan *imap.Message, 1)

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uid)

	go func() {
		if err := c.UidFetch(seqSet, items, messages); err != nil {
			log.Fatal(err)
		}
	}()

	msg := <-messages
	if msg == nil {
		return errors.New("Server didn't return message")
	}

	r := msg.GetBody(section)
	if r == nil {
		return errors.New("Server didn't return message body")
	}

	md5hash := md5.New()

	tmpFilename := fmt.Sprintf("%d_%d.%d.%s,U=%d", time.Now().Unix(), <-h.seqNumChan, h.processID, h.hostname, uid)
	mailboxPath := filepath.Join(h.maildirPath, mailbox)
	tmpPath := filepath.Join(mailboxPath, "tmp", tmpFilename)

	err = os.MkdirAll(filepath.Join(mailboxPath, "tmp"), 0700)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Join(mailboxPath, "cur"), 0700)
	if err != nil {
		return err
	}

	fd, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	multiwriter := io.MultiWriter(fd, md5hash)
	_, err = io.Copy(multiwriter, r)
	if err != nil {
		// Perform cleanup
		_ = fd.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	_ = fd.Close()

	sum := fmt.Sprintf("%x", md5hash.Sum(nil))
	newFilename := fmt.Sprintf("%s,FMD5=%s", tmpFilename, sum)
	newPath := filepath.Join(mailboxPath, "cur", newFilename)
	err = os.Rename(tmpPath, newPath)
	if err != nil {
		// Could not rename file - discard old entry to avoid duplicates
		_ = os.Remove(tmpPath)
		return err
	}

	// Add file to index
	m, st := h.db.AddMessage(newPath)
	if st == notmuch.STATUS_DUPLICATE_MESSAGE_ID {
		// We've already seen this one
		if m != nil {
			m.Destroy()
		}
		return nil
	}

	if st != notmuch.STATUS_SUCCESS {
		if m != nil {
			m.Destroy()
		}
		return errors.New(st.String())
	}

	// If we haven't seen it before, add an "unread" tag to it
	m.AddTag("unread")

	// Add all messages to inbox
	m.AddTag("inbox")

	// Add additional tags specified in config file
	if extraTags, ok := h.mailbox.FolderTags[mailbox]; ok {
		for _, tag := range strings.Split(extraTags, ",") {
			if strings.HasPrefix(tag, "-") {
				m.RemoveTag(tag[1:])
			} else {
				m.AddTag(tag)
			}
		}
	}

	tags := m.GetTags()
	tagnames := []string{}
	for tags.Valid() {
		tagnames = append(tagnames, tags.String())
		tags.MoveToNext()
	}
	if len(tagnames) > 0 {
		fmt.Printf(" tagging %s: %s\n", tmpFilename, strings.Join(tagnames, ","))
	}
	m.Destroy()

	//// Create a new mail reader
	//mr, err := mail.CreateReader(r)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//
	//// Print some info about the message
	//header := mr.Header
	//if date, err := header.Date(); err == nil {
	//	log.Println("Date:", date)
	//}
	//if from, err := header.AddressList("From"); err == nil {
	//	log.Println("From:", from)
	//}
	//if to, err := header.AddressList("To"); err == nil {
	//	log.Println("To:", to)
	//}
	//if subject, err := header.Subject(); err == nil {
	//	log.Println("Subject:", subject)
	//}
	//
	//// Process each message's part
	//for {
	//	p, err := mr.NextPart()
	//	if err == io.EOF {
	//		break
	//	} else if err != nil {
	//		log.Fatal(err)
	//	}
	//
	//	switch h := p.Header.(type) {
	//	case mail.TextHeader:
	//		// This is the message's text (can be plain-text or HTML)
	//		b, _ := ioutil.ReadAll(p.Body)
	//		log.Println("Got text: %v", string(b))
	//	case mail.AttachmentHeader:
	//		// This is an attachment
	//		filename, _ := h.Filename()
	//		log.Println("Got attachment: %v", filename)
	//	}
	//}
	return nil
}

// GetLastFetched returns the timestamp when we last checked this mailbox
func (h *Handler) getLastSeenUID(mailbox string) uint32 {
	if uid, ok := h.cfg.LastSeenUID[mailbox]; ok {
		return uid
	}
	return 0
}

func (h *Handler) setLastSeenUID(mailbox string, uid uint32) {
	h.cfg.LastSeenUID[mailbox] = uid
}

// seenMessage returns true if we've already seen this message
func (h *Handler) seenMessage(messageID string) bool {
	// Remove surrounding tags
	if (strings.HasPrefix(messageID, "<") && strings.HasSuffix(messageID, ">")) ||
		(strings.HasPrefix(messageID, "\"") && strings.HasSuffix(messageID, "\"")) {
		messageID = messageID[1 : len(messageID)-1]
	}

	queryStr := fmt.Sprintf("id:\"%s\"", strings.Replace(messageID, "\"", "\\\"", -1))
	q := h.db.CreateQuery(queryStr)
	matching := q.CountMessages()
	q.Destroy()
	if matching > 0 {
		return true
	}
	return false
}

func (h *Handler) mailboxFetchMessages(c *client.Client, mailbox string) error {
	mbox, err := c.Select(mailbox, false)
	if err != nil {
		return err
	}

	if mbox.Messages == 0 {
		return nil
	}

	// Search for new UID's
	seqSet := new(imap.SeqSet)
	lastSeenUID := h.getLastSeenUID(mailbox)
	// Note that we search from lastSeenUID to MAX, instead of
	//   lastSeenUID to '*', because the latter always returns at least one entry
	seqSet.AddRange(lastSeenUID+1, math.MaxUint32)

	// Fetch envelope information (contains messageid, and UID, which we'll use to fetch the body
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchUid}

	messages := make(chan *imap.Message, 100)
	errchan := make(chan error, 1)

	go func() {
		if err := c.UidFetch(seqSet, items, messages); err != nil {
			errchan <- err
		}
	}()

	var uidList []uint32
	for msg := range messages {
		if msg == nil {
			// We're done
			break
		}

		if msg.Envelope == nil {
			return errors.New("server returned empty envelope")
		}

		if msg.Uid > lastSeenUID {
			lastSeenUID = msg.Uid
		}

		if h.seenMessage(msg.Envelope.MessageId) {
			// We've already seen this message
			fmt.Println("Already seen", msg.Uid, msg.Envelope.MessageId)
			continue
		}
		fmt.Println("Adding to list", msg.Uid, msg.Envelope.MessageId)
		uidList = append(uidList, msg.Uid)
	}

	// Check if an error occurred while fetching data
	select {
	case err := <-errchan:
		return err
	default:
	}

	for _, uid := range uidList {
		err = h.getMessage(c, mailbox, uid)
		if err != nil {
			return err
		}
	}
	h.setLastSeenUID(mailbox, lastSeenUID)
	return nil
}

func (h *Handler) listFolders(c *client.Client) ([]string, error) {

	includeAll := false
	// If no specific folders are listed to be included, assume all folders should be included
	if len(h.mailbox.Folders.Include) == 0 {
		includeAll = true
	}

	// Make a map of included and excluded mailboxes
	includedFolders := make(map[string]bool)
	for _, folder := range h.mailbox.Folders.Include {
		// Note - we set this to false to keep track of if it exists on the server or not
		includedFolders[folder] = false
	}

	excludedFolders := make(map[string]bool)
	for _, folder := range h.mailbox.Folders.Exclude {
		excludedFolders[folder] = true
	}

	mboxChan := make(chan *imap.MailboxInfo, 10)
	errChan := make(chan error, 1)
	go func() {
		if err := c.List("", "*", mboxChan); err != nil {
			errChan <- err
		}
	}()

	var folderNames []string
	for mb := range mboxChan {
		if mb == nil {
			// We're done
			break
		}

		// Check if this mailbox should be excluded
		if _, ok := excludedFolders[mb.Name]; ok {
			continue
		}

		if !includeAll {
			if _, ok := includedFolders[mb.Name]; !ok {
				continue
			}
			includedFolders[mb.Name] = true
		}

		folderNames = append(folderNames, mb.Name)
	}

	// Check if an error occurred while fetching data
	select {
	case err := <-errChan:
		return nil, err
	default:
	}

	// Check if any of the specified folders were missing on the server
	for folder, seen := range includedFolders {
		if !seen {
			return nil, fmt.Errorf("folder %s not found on server", folder)
		}
	}

	return folderNames, nil
}

// CheckMessages checks for new/unindexed messages on the server
func (h *Handler) CheckMessages() error {
	var c *client.Client
	var err error

	if h.mailbox.Server == "" {
		return errors.New("imap server address not configured")
	}
	if h.mailbox.Username == "" {
		return errors.New("imap username not configured")
	}
	if h.mailbox.Password == "" {
		return errors.New("imap password not configured")
	}

	// Set default port
	if h.mailbox.Port == 0 {
		h.mailbox.Port = 143
		if h.mailbox.UseTLS {
			h.mailbox.Port = 993
		}
	}

	connectionString := fmt.Sprintf("%s:%d", h.mailbox.Server, h.mailbox.Port)
	tlsConfig := &tls.Config{ServerName: h.mailbox.Server}
	if h.mailbox.UseTLS {
		c, err = client.DialTLS(connectionString, tlsConfig)
	} else {
		c, err = client.Dial(connectionString)
	}

	if err != nil {
		return err
	}

	// Don't forget to logout
	defer c.Logout()

	// Start a TLS session
	if h.mailbox.UseStartTLS {
		if err := c.StartTLS(tlsConfig); err != nil {
			return err
		}
	}

	err = c.Login(h.mailbox.Username, h.mailbox.Password)
	if err != nil {
		return err
	}

	mailboxes, err := h.listFolders(c)
	for _, mb := range mailboxes {
		err = h.mailboxFetchMessages(c, mb)
		if err != nil {
			return err
		}
	}
	return nil
}
