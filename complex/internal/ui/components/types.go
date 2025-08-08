package components

// PanelDimensions represents computed sizes for main panels
type PanelDimensions struct {
    ConversationWidth  int
    ConversationHeight int
    SidebarWidth       int
    SidebarHeight      int
}

// ConversationConstraints captures limits for conversation rendering
type ConversationConstraints struct {
    MaxHeight          int
    ViewportHeight     int
    ScrollSpaceHeight  int
    ConversationWidth  int
    ConversationHeight int
}

// PanelContent is used for validating layout sizes
type PanelContent struct {
    ID      string
    Content []string
    Height  int
}

