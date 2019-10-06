package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/jhillyerd/enmime"
	"github.com/jroimartin/gocui"
	"github.com/yzzyx/mr/models"
)

type threadMessageInfo struct {
	models.Message
	expanded  bool
	lineCount int
	envelope  *enmime.Envelope
	lines     []string
}

type ThreadView struct {
	messages []threadMessageInfo
	thread   models.Thread
}

func NewThreadView(thread models.Thread) (*ThreadView, error) {
	v := &ThreadView{}
	v.messages = make([]threadMessageInfo, 0, len(v.messages))
	v.thread = thread

	linenumber := 0
	for _, m := range thread.Messages {

		f, err := os.Open(m.Filename)
		if err != nil {
			return v, err
		}

		env, err := enmime.ReadEnvelope(f)
		if err != nil {
			return v, err
		}

		lines := []string{}
		for _, hdr := range []string{"Subject", "From", "To", "Cc", "Bcc", "Date"} {
			line := env.GetHeader(hdr)
			if line == "" {
				continue
			}
			lines = append(lines, " │ "+hdr+": "+line)
		}

		content := strings.Split(env.Text, "\n")
		for k := range content {
			lines = append(lines, " │ "+strings.ReplaceAll(content[k], "\r", ""))
		}
		v.messages = append(v.messages, threadMessageInfo{
			Message:   m,
			expanded:  false,
			lineCount: 1,
			envelope:  env,
			lines:     lines,
		})
		linenumber++

	}
	return v, nil
}

func (v *ThreadView) GetLine(lineNumber int) (string, error) {
	count := 0
	for _, m := range v.messages {
		// It's the header for one of the messages
		if count == lineNumber {
			return m.Label(), nil
		}

		// It's not in this entry
		if lineNumber >= count+m.lineCount {
			count += m.lineCount
			continue
		}

		partLine := lineNumber - count - 1
		if partLine >= len(m.lines) {
			return "invalid line number", nil
		}
		return m.lines[partLine], nil
	}
	return "linenumber not in any message", nil
}

func (v *ThreadView) GetMaxLines() (int, error) {
	count := 0
	for _, m := range v.messages {
		count += m.lineCount
	}
	return count, nil
}

func (v *ThreadView) GetLabel() (string, error) {
	return v.thread.Subject, nil
}

func (v *ThreadView) HandleKey(ui *UI, key interface{}, mod gocui.Modifier, lineNumber int) error {
	// Handle enter
	if k, ok := key.(gocui.Key); ok && k == gocui.KeyEnter {
		count := 0
		for k := range v.messages {
			// Expand or contract email
			if lineNumber == count {
				v.messages[k].ToggleExpanded()

				break
			}
			count += v.messages[k].lineCount
		}
	}
	return nil
}

func (m *threadMessageInfo) ToggleExpanded() {
	m.expanded = !m.expanded

	if !m.expanded {
		m.lineCount = 1
		return
	} else {
		m.lineCount = len(m.lines) + 1
	}
}

func (m threadMessageInfo) Label() string {
	timeFormat := "2006-01-02 15:04"
	timeStr := m.Date.Format(timeFormat)
	timeLen := len(timeFormat)
	authorLen := 25

	// FIXME - 8bit or 256bit colors?
	//line := fmt.Sprintf("\x1b[3%d;%dm", 7, 1)

	line := fmt.Sprintf("\x1b[48;5;%dm\x1b[30m", 204)
	line += fmt.Sprintf("%-[2]*.[2]*[1]s ", timeStr, timeLen)
	line += fmt.Sprintf("%-[2]*.[2]*[1]s ", m.envelope.GetHeader("From"), authorLen)
	line += m.envelope.GetHeader("Subject")
	line += "\x1b[0m"

	return line
}
