package ui

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/jhillyerd/enmime"
	"github.com/jroimartin/gocui"
)

type MailView struct {
	envelope *enmime.Envelope
	isReply  bool
	lines    []string
}

type MailSettings struct {
	To              string
	ReplyToFilename string
}

var mailHeaders = []string{
	"Subject",
	"To",
	"From",
	"Cc",
	"Bcc",
	"ReplyTo",
}

func NewMailView(m MailSettings) (*MailView, error) {
	var err error

	v := &MailView{}
	v.lines = []string{}
	v.envelope, err = enmime.ReadEnvelope(strings.NewReader(""))
	if err != nil {
		return nil, err
	}

	if m.ReplyToFilename != "" {
		f, err := os.Open(m.ReplyToFilename)
		if err != nil {
			v.lines = []string{
				fmt.Sprintf("Cannot open file %s:", m.ReplyToFilename),
				err.Error(),
			}
			return v, nil
		}

		v.envelope, err = enmime.ReadEnvelope(f)
		if err != nil {
			v.lines = []string{
				fmt.Sprintf("Cannot read mail from file %s:", m.ReplyToFilename),
				err.Error(),
			}
			return v, nil
		}
		v.isReply = true
	}

	if m.To != "" {
		err := v.envelope.SetHeader("To", []string{m.To})
		if err != nil {
			return nil, err
		}
	}

	v.lines = append(v.lines, strings.Split(v.envelope.Text, "\n")...)
	return v, nil
}

func (v *MailView) GetLine(lineNumber int) (string, error) {
	// map lines to headers
	headerColumnLen := 7 // length of longest header name

	if lineNumber < len(mailHeaders) {
		return fmt.Sprintf("%[2]*[1]s: %[3]s", mailHeaders[lineNumber], headerColumnLen, v.envelope.GetHeader(mailHeaders[lineNumber])), nil

	}
	return v.lines[lineNumber-len(mailHeaders)], nil
}

func (v *MailView) GetMaxLines() (int, error) {
	return len(mailHeaders) + len(v.lines), nil
}

func (v *MailView) GetLabel() (string, error) {
	if v.isReply {
		return "Reply to " + v.envelope.GetHeader("Subject"), nil
	}
	return "New email", nil
}

func (v *MailView) HandleKey(ui *UI, key interface{}, mod gocui.Modifier, lineNumber int) error {

	// On enter, launch editor
	if k, ok := key.(gocui.Key); ok && k == gocui.KeyEnter {

		file, err := ioutil.TempFile("", "mail*.eml")
		if err != nil {
			return err
		}

		for _, h := range mailHeaders {
			_, err = fmt.Fprintf(file, "%s: %s\n", h, v.envelope.GetHeader(h))
			if err != nil {
				return err
			}
		}

		_, err = fmt.Fprintln(file, v.envelope.Text)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, file.Name())
		cmd := exec.Command("vi", file.Name())
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s", err)
			return err
		}

	}

	return nil
}
