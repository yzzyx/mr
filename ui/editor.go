package ui

import "github.com/jroimartin/gocui"

// EditorOnCLose defines a callback used in editors
type EditorOnClose func(ok bool, s string)

// SingleLineEditor implements the Editor interface, and
// can be used to edit single line entries.
// When Enter is pressed, OnClose() will be called, with ok set to true, and contents in s
// If Esc is pressed, OnClose() will be called with ok set to false
type SingleLineEditor struct {
	OnClose EditorOnClose
}

func (ed *SingleLineEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch {
	case ch != 0 && ch >= 0x20 && mod == 0:
		v.EditWrite(ch)
	case key == gocui.KeyEnter:
		data := v.ViewBufferLines()[0]
		ed.OnClose(true, data)
	case key == gocui.KeyEsc:
		data := v.ViewBufferLines()[0]
		ed.OnClose(false, data)
	case key == gocui.KeySpace:
		v.EditWrite(' ')
	case key == gocui.KeyBackspace || key == gocui.KeyBackspace2:
		v.EditDelete(true)
	case key == gocui.KeyArrowLeft:
		v.MoveCursor(-1, 0, false)
	case key == gocui.KeyArrowRight:
		v.MoveCursor(1, 0, false)
	}
}
