package ui

import (
	"github.com/jroimartin/gocui"
)

func (ui *UI) keyUp(g *gocui.Gui, v *gocui.View) error {
	return ui.currentView.UpdateLinePos(-1)
}

func (ui *UI) keyDown(g *gocui.Gui, v *gocui.View) error {
	return ui.currentView.UpdateLinePos(1)
}

func (ui *UI) pageUp(g *gocui.Gui, v *gocui.View) error {
	return ui.currentView.UpdateLinePos(-20)
}

func (ui *UI) pageDown(g *gocui.Gui, v *gocui.View) error {
	return ui.currentView.UpdateLinePos(20)
}

func (ui *UI) handleKey(key interface{}, mod gocui.Modifier) func(g *gocui.Gui, v *gocui.View) error {

	return func(g *gocui.Gui, v *gocui.View) error {
		// What happens to enter is defined by the current scroller and line,
		// so we'll just defer that information to the underlying scroller

		selectedLine := ui.currentView.GetSelectedLine()
		return ui.currentView.HandleKey(ui, key, mod, selectedLine)
	}
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func (ui *UI) KeyBindings(g *gocui.Gui) error {
	if err := g.SetKeybinding("main", gocui.KeyArrowDown, gocui.ModNone, ui.keyDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyArrowUp, gocui.ModNone, ui.keyUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyPgup, gocui.ModNone, ui.pageUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyPgdn, gocui.ModNone, ui.pageDown); err != nil {
		return err
	}

	keys := []struct {
		key interface{}
		mod gocui.Modifier
	}{
		{key: gocui.KeyEnter},
		{key: 't'},
		{key: '/'},
		{key: 'm'},
	}

	for _, k := range keys {
		if err := g.SetKeybinding("main", k.key, k.mod, ui.handleKey(k.key, k.mod)); err != nil {
			return err
		}
	}

	if err := g.SetKeybinding("main", gocui.KeyTab, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { ui.NextView(); return nil }); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", 'x', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { return ui.CloseView() }); err != nil {
		return err
	}

	// Editor keybindings
	if err := g.SetKeybinding("main", gocui.KeyInsert, gocui.ModNone, ui.editView); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}

	return nil
}
