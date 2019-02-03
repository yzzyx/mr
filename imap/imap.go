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

	"github.com/spf13/viper"

	"github.com/yzzyx/mr/notmuch"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

func init() {
	viper.SetDefault("imap.server", "")
	viper.SetDefault("imap.username", "")
	viper.SetDefault("imap.password", "")
	viper.SetDefault("imap.use_tls", true)
	viper.SetDefault("imap.use_starttls", false)
}

type mailConfig struct {
	// Keep track of last seen UID for each mailbox
	LastSeenUID map[string]uint32
}

type IndexUpdate struct {
	Path      string   // Path to file to be updated
	MessageID string   // MessageID to be updated
	Tags      []string // Tags to add/remove from message (entries prefixed with "-" will be removed)
}

type IMAPHandler struct {
	db          *notmuch.Database
	maildirPath string
	configPath  string

	cfg mailConfig

	// Used internally to generate maildir files
	seqNumChan <-chan int
	processId  int
	hostname   string
}

// New creates a new IMAPHandler
func New(db *notmuch.Database, maildirPath string, configPath string) (*IMAPHandler, error) {
	var err error
	h := IMAPHandler{}
	h.hostname, err = os.Hostname()
	if err != nil {
		return nil, err
	}

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
	h.processId = os.Getpid()
	h.db = db
	h.maildirPath = maildirPath
	h.configPath = configPath

	h.cfg.LastSeenUID = make(map[string]uint32)
	// Get list of timestamps etc.
	data, err := ioutil.ReadFile(filepath.Join(configPath, "imap-uids"))
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
func (h *IMAPHandler) Close() error {
	data, err := json.Marshal(h.cfg)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(h.configPath, "imap-uids"), data, 0700)
	if err != nil {
		return err
	}

	return nil
}

// getMessage downloads a message from the server from a mailbox, and stores it in a maildir
func (h *IMAPHandler) getMessage(c *client.Client, mailbox string, uid uint32) error {
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

	tmpFilename := fmt.Sprintf("%d_%d.%d.%s,U=%d", time.Now().Unix(), <-h.seqNumChan, h.processId, h.hostname, uid)
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

	extraTags := viper.GetString("imap.mailboxes." + mailbox + ".default_tags")
	for _, tag := range strings.Split(extraTags, ",") {
		if strings.HasPrefix(tag, "-") {
			m.RemoveTag(tag[1:])
		} else {
			m.AddTag(tag)
		}
	}

	tags := m.GetTags()
	tagnames := []string{}
	for tags.Valid() {
		tagnames = append(tagnames, tags.String())
		tags.MoveToNext()
	}
	if len(tagnames) > 0 {
		fmt.Printf(" tags: %s\n", strings.Join(tagnames, ","))
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
func (h *IMAPHandler) getLastSeenUID(mailbox string) uint32 {
	if uid, ok := h.cfg.LastSeenUID[mailbox]; ok {
		return uid
	}
	return 0
}

func (h *IMAPHandler) setLastSeenUID(mailbox string, uid uint32) {
	h.cfg.LastSeenUID[mailbox] = uid
}

// seenMessage returns true if we've already seen this message
func (h *IMAPHandler) seenMessage(messageID string) bool {
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

func (h *IMAPHandler) mailboxFetchMessages(c *client.Client, mailbox string) error {
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

func (h *IMAPHandler) listMailboxes(c *client.Client) ([]string, error) {
	// Make a map of excluded mailboxes
	excludedList := viper.GetStringSlice("imap.excluded")
	excludedMailboxes := make(map[string]bool)
	for _, mb := range excludedList {
		excludedMailboxes[mb] = true
	}

	mboxChan := make(chan *imap.MailboxInfo, 10)
	errChan := make(chan error, 1)
	go func() {
		if err := c.List("", "*", mboxChan); err != nil {
			errChan <- err
		}
	}()

	var mailboxNames []string
	for mb := range mboxChan {
		if mb == nil {
			// We're done
			break
		}

		// Check if this mailbox should be excluded
		if _, ok := excludedMailboxes[mb.Name]; ok {
			continue
		}

		mailboxNames = append(mailboxNames, mb.Name)
	}

	// Check if an error occurred while fetching data
	select {
	case err := <-errChan:
		return nil, err
	default:
	}

	return mailboxNames, nil
}

// CheckMessages checks for new/unindexed messages on the server
func (h *IMAPHandler) CheckMessages() error {
	var c *client.Client
	var err error

	serverAddr := viper.GetString("imap.server")
	portNum := viper.GetInt("imap.port")
	username := viper.GetString("imap.username")
	password := viper.GetString("imap.password")

	if serverAddr == "" {
		return errors.New("imap server address not configured")
	}
	if username == "" {
		return errors.New("imap username not configured")
	}
	if password == "" {
		return errors.New("imap password not configured")
	}

	connectionString := fmt.Sprintf("%s:%d", serverAddr, portNum)
	if viper.GetBool("imap.use_tls") {
		tlsConfig := &tls.Config{ServerName: serverAddr}
		c, err = client.DialTLS(connectionString, tlsConfig)
	} else {
		c, err = client.Dial(connectionString)
	}
	// Don't forget to logout
	defer c.Logout()

	// Start a TLS session
	if viper.GetBool("imap.use_starttls") {
		tlsConfig := &tls.Config{ServerName: serverAddr}
		if err := c.StartTLS(tlsConfig); err != nil {
			return err
		}
	}

	err = c.Login(username, password)
	if err != nil {
		return err
	}

	mailboxes, err := h.listMailboxes(c)
	for _, mb := range mailboxes {
		err = h.mailboxFetchMessages(c, mb)
		if err != nil {
			return err
		}
	}
	return nil

}
