package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/thereisnotime/x11droid/internal/system"
	"github.com/thereisnotime/x11droid/internal/tui"
)

func main() {
	sess := system.Detect()
	if w := sess.Warning(); w != "" {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}

	p := tea.NewProgram(tui.New(sess), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
