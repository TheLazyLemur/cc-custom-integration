package components

// LayoutManager centralizes layout calculations and constraints
type LayoutManager struct {
    width               int
    height              int
    headerFooterMargin  int // combined header, footer, margins (existing code uses 4)
    panelPaddingMargin  int // extra padding/margins inside panels (existing code used -4)
    sidebarWidthTotal   int // total sidebar reservation (style width + margins)
    scrollIndicatorLines int // reserved lines for scroll indicator
}

// NewLayoutManager creates a new layout manager with defaults matching current UI
func NewLayoutManager(width, height int) *LayoutManager {
    return &LayoutManager{
        width:                width,
        height:               height,
        headerFooterMargin:   4,  // from renderMainView: contentHeight := a.height - 4
        panelPaddingMargin:   4,  // renderConversationPanel called with height-4
        sidebarWidthTotal:    35, // leftWidth := a.width - 35
        scrollIndicatorLines: 2,  // reserved for scroll status
    }
}

// CalculatePanelDimensions returns the sizes to use for panels
func (lm *LayoutManager) CalculatePanelDimensions() PanelDimensions {
    // Available height for the main content area
    contentHeight := lm.height - lm.headerFooterMargin
    // Panel heights used in current implementation
    panelHeight := contentHeight - lm.panelPaddingMargin

    // Widths: conversation takes remaining width after sidebar reservation
    convWidth := lm.width - lm.sidebarWidthTotal
    if convWidth < 1 {
        convWidth = 1
    }

    // Sidebar style sets Width(30); we reserve 35 in total to include spacing
    sidebarWidth := lm.sidebarWidthTotal

    if panelHeight < 1 {
        panelHeight = 1
    }

    return PanelDimensions{
        ConversationWidth:  convWidth,
        ConversationHeight: panelHeight,
        SidebarWidth:       sidebarWidth,
        SidebarHeight:      panelHeight,
    }
}

// GetConversationConstraints computes rendering constraints for the conversation area
func (lm *LayoutManager) GetConversationConstraints() ConversationConstraints {
    dims := lm.CalculatePanelDimensions()
    // Inner content height for conversation (account for panel padding/border ~4)
    inner := dims.ConversationHeight - lm.panelPaddingMargin
    if inner < 1 {
        inner = 1
    }
    viewport := inner - lm.scrollIndicatorLines
    if viewport < 1 {
        viewport = 1
    }
    return ConversationConstraints{
        MaxHeight:          inner,
        ViewportHeight:     viewport,
        ScrollSpaceHeight:  lm.scrollIndicatorLines,
        ConversationWidth:  dims.ConversationWidth,
        ConversationHeight: dims.ConversationHeight,
    }
}

// ValidatePanelHeights ensures panels do not exceed allocated heights
func (lm *LayoutManager) ValidatePanelHeights(panels []PanelContent) error {
    // Minimal validation placeholder; can be expanded to detailed checks
    // Currently no-op to avoid introducing new error paths before full integration
    return nil
}
