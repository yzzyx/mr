package ui

import (
	"fmt"
	"strings"

	"github.com/jroimartin/gocui"
	"github.com/yzzyx/mr/models"
)

type ListView struct {
	search     string
	query      *models.Query
	rangeStart int
	rangeEnd   int
}

func NewListView(query string) (*ListView, error) {
	view := &ListView{}
	view.search = query
	view.query = models.NewQuery(query)
	return view, nil
}

func (v *ListView) GetLine(lineNumber int) (string, error) {
	t := v.query.GetLine(lineNumber)

	line := " "
	//if l.tagged {
	//	line = "*"
	//}
	line += strings.Join(t.Tags, ",")
	line += " " + t.Authors
	line += " " + t.Subject
	return line, nil
}

func (v *ListView) GetMaxLines() (int, error) {
	return v.query.Count(), nil
}

func (v *ListView) GetLabel() (string, error) {
	if v.search != "" {
		return "search: " + v.search, nil
	}
	return "list", nil
}

func (v *ListView) editor(ui *UI, content string, onClose EditorOnClose) error {
	g := ui.gui
	maxX, maxY := g.Size()
	view, err := g.SetView("edit", 1, maxY/2-1, maxX, maxY/2+1)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}

	view.Overwrite = false
	view.Frame = true
	view.Editable = true
	fmt.Fprint(view, content)
	err = view.SetCursor(len(content), 0)
	if err != nil {
		return err
	}

	// Show cursor while editing
	g.Cursor = true
	view.Editor = &SingleLineEditor{
		OnClose: func(ok bool, s string) {
			_ = g.DeleteView("edit")
			g.Cursor = false
			_, _ = g.SetCurrentView("main")
			onClose(ok, s)
		},
	}

	_, err = g.SetViewOnTop("edit")
	if err != nil {
		return err
	}

	_, err = g.SetCurrentView("edit")
	return err
}

func (v *ListView) editTags(ui *UI, lineNumber int) error {
	thread := v.query.GetLine(lineNumber)
	tags := strings.Join(thread.Tags, ",")
	return v.editor(ui, tags, func(ok bool, newTags string) {
		if !ok {
			return
		}

		tagList := strings.Split(newTags, ",")
		thread.Tags = make([]string, 0, len(tagList))
		for _, nt := range tagList {
			nt = strings.TrimSpace(nt)
			if nt == "" {
				continue
			}
			thread.Tags = append(thread.Tags, nt)
		}
	})
}

func (v *ListView) showSearch(ui *UI) error {
	return v.editor(ui, "", func(ok bool, search string) {
		if !ok {
			return
		}

		newLst, err := NewListView(search)
		if err != nil {
			return
		}
		ui.AddView(NewScroller(newLst))
	})
}

func (v *ListView) HandleKey(ui *UI, key interface{}, mod gocui.Modifier, lineNumber int) error {
	// Handle enter
	if k, ok := key.(gocui.Key); ok && k == gocui.KeyEnter {
		thread := v.query.GetLine(lineNumber)
		if len(thread.Messages) == 0 {
			return nil
		}

		filename := thread.Messages[0]
		content, err := NewMailView(filename)
		if err != nil {
			return err
		}

		view := NewScroller(content)
		ui.AddView(view)
	}

	if k, ok := key.(rune); ok {
		switch k {
		case '*': // star message
		// FIXME - tagged
		//v.lines[lineNumber].tagged = !v.lines[lineNumber].tagged
		case 't': // tag message
			return v.editTags(ui, lineNumber)
		case '/': // search for messages
			return v.showSearch(ui)
		}

	}

	return nil
}
