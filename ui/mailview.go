package ui

import (
	"os"
	"strings"

	"github.com/jaytaylor/html2text"
	"github.com/jhillyerd/enmime"
	"github.com/jroimartin/gocui"
)

type MailView struct {
	lines    []string
	envelope *enmime.Envelope
}

func NewMailView(filename string) (*MailView, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	env, err := enmime.ReadEnvelope(f)
	if err != nil {
		return nil, err
	}

	v := &MailView{}
	v.lines = []string{}

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

func (v *MailView) GetLine(lineNumber int) (string, error) {
	return v.lines[lineNumber], nil
}

func (v *MailView) GetMaxLines() (int, error) {
	return len(v.lines), nil
}

func (v *MailView) GetLabel() (string, error) {
	return v.envelope.GetHeader("Subject"), nil
}

func (v *MailView) HandleKey(ui *UI, key interface{}, mod gocui.Modifier, lineNumber int) error {
	// Ignore all keys
	return nil
}
