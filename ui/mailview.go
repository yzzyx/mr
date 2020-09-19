package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/jaytaylor/html2text"
	"github.com/jhillyerd/enmime"
	"github.com/jroimartin/gocui"
)

// MailView displays an email message
type MailView struct {
	lines    []string
	envelope *enmime.Envelope
}

// NewMailView creates a new MailView, with the contents from 'filename'
func NewMailView(filename string) (*MailView, error) {
	v := &MailView{}
	v.lines = []string{}

	f, err := os.Open(filename)
	if err != nil {
		v.lines = []string{
			fmt.Sprintf("Cannot open file %s:", filename),
			err.Error(),
		}
		return v, nil
	}

	env, err := enmime.ReadEnvelope(f)
	if err != nil {
		v.lines = []string{
			fmt.Sprintf("Cannot read mail from file %s:", filename),
			err.Error(),
		}
		return v, nil
	}

	for _, hdr := range []string{"Subject", "From", "To", "Cc", "Bcc", "Date"} {
		line := env.GetHeader(hdr)
		if line == "" {
			continue
		}
		v.lines = append(v.lines, hdr+": "+line)
	}

	v.lines = append(v.lines, strings.Split(env.Text, "\n")...)
	v.lines = append(v.lines, "==========")

	html, err := html2text.FromString(env.HTML, html2text.Options{PrettyTables: true})
	if err != nil {
		return nil, err
	}
	v.lines = append(v.lines, strings.Split(html, "\n")...)

	v.envelope = env
	return v, nil
}

// GetLine returns the contents of a specific line in the mailview
func (v *MailView) GetLine(lineNumber int) (string, error) {
	return v.lines[lineNumber], nil
}

// GetMaxLines returns the number of lines in the message
func (v *MailView) GetMaxLines() (int, error) {
	return len(v.lines), nil
}

// GetLabel returns the label for the mailview
func (v *MailView) GetLabel() (string, error) {
	return v.envelope.GetHeader("Subject"), nil
}

// HandleKey updates the mailview based on key input
func (v *MailView) HandleKey(ui *UI, key interface{}, mod gocui.Modifier, lineNumber int) error {
	// Ignore all keys
	return nil
}
