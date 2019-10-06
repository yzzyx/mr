package ui

import "github.com/jroimartin/gocui"

// Content defines the interface used to show contents in a Scroller
type Content interface {
	GetLabel() (string, error)
	GetLine(lineNumber int) (string, error)
	GetMaxLines() (int, error)
	HandleKey(ui *UI, key interface{}, mod gocui.Modifier, lineNumber int) error
}

// Scroller defines a scrollable scroller
type Scroller struct {
	contents     Content
	selectedLine int
	startLine    int
	endLine      int
	height       int
	maxLines     int
}

// NewScroller creates a new Scroller with a specific content
func NewScroller(contents Content) *Scroller {
	view := &Scroller{}
	view.contents = contents
	return view
}

// GetLine returns the specified line from contents
func (v *Scroller) GetLine(lineNumber int) (string, error) {
	return v.contents.GetLine(lineNumber)
}

// GetLabel returns the label from contents
func (v *Scroller) GetLabel() (string, error) {
	return v.contents.GetLabel()
}

// GetSelectedLine returns the currently selected line
func (v *Scroller) GetSelectedLine() int {
	return v.selectedLine
}

// GetStartLine returns the current start line
func (v *Scroller) GetStartLine() int {
	return v.startLine
}

// GetEndLine returns the current end line
func (v *Scroller) GetEndLine() int {
	return v.endLine
}

func (v *Scroller) HandleKey(ui *UI, key interface{}, mod gocui.Modifier, lineNumber int) error {
	err := v.contents.HandleKey(ui, key, mod, lineNumber)
	if err != nil {
		return err
	}

	// Check if we need to update start- or endline properties
	return v.UpdateHeight(v.height)
}

// UpdateHeight recalculates start/end, and make sure that selected line is still visible
func (v *Scroller) UpdateHeight(height int) error {
	// Check if maxlines have changed
	maxLines, err := v.contents.GetMaxLines()
	if err != nil {
		return err
	}
	maxLines-- // we need a 0-indexed number

	if height == v.height && maxLines == v.maxLines {
		return nil
	}

	if height < v.height {
		if v.startLine+height <= v.selectedLine {
			// Make sure that currently selected line is still
			// visible (remove lines from the top)
			v.endLine = v.selectedLine
			v.startLine = v.endLine - height
		} else {
			v.endLine = v.startLine + height
		}
	} else if height > v.height || maxLines > v.maxLines {
		v.endLine = v.startLine + height
	}
	v.height = height
	v.maxLines = maxLines

	if v.endLine > maxLines {
		v.endLine = maxLines
		v.startLine = v.endLine - height
		if v.startLine < 0 {
			v.startLine = 0
		}
	}
	return nil
}

// UpdateLinePos moves the currently selected line up or down according to 'dy'
// dy < 0 moves the currently selected line up,
// dy > 0 moves the currently selected line down
func (v *Scroller) UpdateLinePos(dy int) error {
	// Update selected position
	v.selectedLine += dy

	// Handle scrolling
	if dy < 0 {
		// We're moving up
		if v.selectedLine < 0 {
			v.selectedLine = 0
		}

		if v.selectedLine < v.startLine {
			v.startLine = v.selectedLine
			v.endLine = v.startLine + v.height
		}

	} else if dy > 0 {
		// we're moving down
		maxlines, err := v.contents.GetMaxLines()
		if err != nil {
			panic(err)
			return err
		}
		maxlines-- // maxlines is calculated as number of lines, but we want a 0-indexed entry

		if v.selectedLine > maxlines {
			v.selectedLine = maxlines
		}

		if v.selectedLine > v.endLine {
			v.endLine = v.selectedLine
			v.startLine = v.endLine - v.height
		}
	}
	return nil
}
