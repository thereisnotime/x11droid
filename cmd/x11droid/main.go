package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/thereisnotime/x11droid/internal/system"
	"github.com/thereisnotime/x11droid/internal/tui"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "x11droid",
		Short:         "Manage Waydroid instances in Podman on X11",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return launchTUI()
		},
	}

	root.AddCommand(
		cmdList(),
		cmdSpawn(),
		cmdStart(),
		cmdStop(),
		cmdRM(),
		cmdLogs(),
		cmdShell(),
		cmdSetup(),
	)

	return root
}

func launchTUI() error {
	sess := system.Detect()
	if w := sess.Warning(); w != "" {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	p := tea.NewProgram(tui.New(sess), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
