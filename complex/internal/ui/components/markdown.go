package components

import (
	"github.com/charmbracelet/glamour"
)

// MarkdownRenderer wraps glamour for consistent markdown rendering
type MarkdownRenderer struct {
	renderer *glamour.TermRenderer
	width    int
}

// NewMarkdownRenderer creates a new markdown renderer with custom styling
func NewMarkdownRenderer(width int) (*MarkdownRenderer, error) {
	// Use the dark style as base and customize it
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(width),
		glamour.WithEmoji(),
	)
	if err != nil {
		return nil, err
	}

	return &MarkdownRenderer{
		renderer: renderer,
		width:    width,
	}, nil
}

// Render renders markdown content to styled terminal output
func (mr *MarkdownRenderer) Render(content string) (string, error) {
	return mr.renderer.Render(content)
}

// UpdateWidth updates the renderer width for responsive display
func (mr *MarkdownRenderer) UpdateWidth(width int) error {
	if width == mr.width {
		return nil
	}
	
	// Recreate renderer with new width
	newRenderer, err := NewMarkdownRenderer(width)
	if err != nil {
		return err
	}
	
	mr.renderer = newRenderer.renderer
	mr.width = width
	return nil
}