package ui

import (
	"fmt"

	"github.com/jroimartin/gocui"
)

type UI struct {
	currentView *Scroller
	views       []*Scroller
	gui         *gocui.Gui
}

func (ui *UI) RenderHeader(g *gocui.Gui) error {
	maxX, _ := g.Size()
	width := int(float32(maxX) / float32(len(ui.views)))
	for idx, uiView := range ui.views {
		xPos := idx * width
		if xPos == 0 {
			// Since we're not drawing frames, place first entry one step further to the left
			xPos = -1
		}

		// Adjust for rounding errors on last entry
		if idx == len(ui.views)-1 {
			if (idx+1)*width <= maxX {
				width = maxX - xPos + 1
			}
		}

		v, err := g.SetView(fmt.Sprintf("header-%d", idx), xPos, -1, xPos+width, 1)
		if err != nil && err != gocui.ErrUnknownView {
			return err
		}

		// Always place headers on top, in order to overwrite existing previously shown entries (after CloseView)
		_, err = g.SetViewOnTop(v.Name())
		if err != nil {
			return err
		}

		v.Frame = false
		v.Autoscroll = false
		v.Wrap = false
		v.Clear()

		if uiView == ui.currentView {
			v.BgColor = gocui.ColorWhite
			v.FgColor = gocui.ColorBlack
		} else {
			v.BgColor = gocui.ColorDefault
			v.FgColor = gocui.ColorDefault
		}

		label, err := uiView.GetLabel()
		if err != nil {
			return err
		}

		_, err = fmt.Fprintf(v, "%[1]d %-[2]*[3]s", idx, width-1, label)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ui *UI) RenderList(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	v, err := g.SetView("main", -1, 0, maxX, maxY+1)
	if err == gocui.ErrUnknownView {
		// Default to this scroller as the current scroller
		_, err = g.SetCurrentView("main")
	}
	if err != nil {
		return err
	}

	maxX -= 2

	//v.Highlight = true
	//v.SelBgColor = gocui.ColorGreen
	//v.SelFgColor = gocui.ColorBlack
	v.Frame = false
	v.Wrap = false

	v.Clear()
	_, viewHeight := v.Size()
	viewHeight -= 2
	err = ui.currentView.UpdateHeight(viewHeight)
	if err != nil {
		return err
	}

	selectedLine := ui.currentView.GetSelectedLine()
	startLine := ui.currentView.GetStartLine()
	endLine := ui.currentView.GetEndLine()

	err = v.SetCursor(0, selectedLine-startLine)
	if err != nil {
		return err
	}

	for i := startLine; i <= endLine; i++ {
		line, err := ui.currentView.GetLine(i)
		if err != nil {
			return err
		}

		marker := "  "
		if i == selectedLine {
			marker = ">>"
		}
		_, err = fmt.Fprintf(v, "%s%-[3]*.[3]*[2]s\n", marker, line, maxX)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ui *UI) Layout(g *gocui.Gui) error {

	err := ui.RenderHeader(g)
	if err != nil {
		return err
	}

	err = ui.RenderList(g)
	return err
}

func (ui *UI) AddView(v *Scroller) {
	ui.views = append(ui.views, v)
	ui.currentView = v
}

func (ui *UI) NextView() {
	var idx int
	for idx = 0; idx < len(ui.views); idx++ {
		if ui.currentView == ui.views[idx] {
			break
		}
	}

	idx++
	if idx == len(ui.views) {
		idx = 0 // wrap around
	}
	ui.currentView = ui.views[idx]
}

func (ui *UI) CloseView() error {
	var idx int

	for idx = 0; idx < len(ui.views); idx++ {
		if ui.currentView == ui.views[idx] {
			copy(ui.views[idx:], ui.views[idx+1:])
			ui.views[len(ui.views)-1] = nil // or the zero value of T
			ui.views = ui.views[:len(ui.views)-1]

			// No views left - quit
			if len(ui.views) == 0 {
				return gocui.ErrQuit
			}

			// Set new current scroller
			if idx == len(ui.views) {
				idx = len(ui.views) - 1
			}
			ui.currentView = ui.views[idx]
			break
		}
	}

	return nil
}

func (ui *UI) editView(g *gocui.Gui, v *gocui.View) error {

	maxX, maxY := g.Size()
	v, err := g.SetView("edit", 1, maxY/2-1, maxX, maxY/2+1)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}

	v.Overwrite = false
	v.Frame = true
	v.Editable = true

	// Show cursor while editing
	g.Cursor = true
	v.Editor = &SingleLineEditor{
		OnClose: func(ok bool, s string) {
			g.DeleteView("edit")
			g.Cursor = false
			_, err = g.SetCurrentView("main")
		},
	}

	_, err = g.SetViewOnTop("edit")
	if err != nil {
		return err
	}

	_, err = g.SetCurrentView("edit")
	return err
}

func Setup() error {
	g, err := gocui.NewGui(gocui.Output256)
	if err != nil {
		return err
	}
	defer g.Close()
	ui := &UI{
		gui: g,
	}

	g.Cursor = false
	g.SetManagerFunc(ui.Layout)

	lv, err := NewListView("")
	if err != nil {
		return err
	}
	ui.AddView(NewScroller(lv))

	if err := ui.KeyBindings(g); err != nil {
		return err
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		return err
	}
	return nil
}
