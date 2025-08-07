# Scrolling Debug Info

## Current Implementation
- Renders all messages into lines first
- Applies line-based offset (scrollPosition)
- Shows viewport of lines from offset

## Viewport Calculation
- Panel height comes in as `contentHeight - 6` (from renderMainView)
- We reserve 2 lines for scroll indicator when scrolling
- Actual viewport = height or height-2 depending on content

## Potential Issues Fixed
1. **Viewport height inconsistency**: Now uses conditional reduction
2. **Scroll indicator placement**: Added as separate lines to finalContent
3. **Line slicing**: Takes exact viewport height of lines

## Test Cases
1. Start TUI with multiple messages
2. Press End to go to bottom - should show last messages
3. Press Home to go to top - should show first messages  
4. Press ↑ to scroll up one line - should shift view up by 1
5. Press ↓ to scroll down one line - should shift view down by 1

## Known Values
- contentHeight = terminal.height - 4
- conversation panel height = contentHeight - 6
- So conversation panel gets: terminal.height - 10