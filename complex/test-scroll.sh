#!/bin/bash
# Test script to verify scrolling works

echo "Testing scrolling in CustomClaude TUI"
echo "======================================"
echo ""
echo "1. Start the TUI: ./complex-tui"
echo "2. Send several messages to build history"
echo "3. Test scrolling:"
echo "   - Use ↑/↓ or j/k to scroll one message at a time"
echo "   - Use PgUp/PgDn to scroll 5 messages at once"
echo "   - Use Home/End to jump to top/bottom"
echo ""
echo "Current implementation:"
echo "- Shows up to 5 messages at once"
echo "- Scroll position indicator at bottom"
echo "- Auto-scrolls to show recent messages"
echo ""
echo "Note: You need at least 6+ messages to see scrolling in action"