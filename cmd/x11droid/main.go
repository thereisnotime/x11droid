// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
	"github.com/spf13/cobra"
	"github.com/thereisnotime/x11droid/internal/system"
	"github.com/thereisnotime/x11droid/internal/tui"
	"github.com/thereisnotime/x11droid/internal/version"
)

// errNotRoot aborts a command that requires root. The message is printed in the
// PreRun hook; this sentinel just makes Execute return non-zero (errors are
// silenced on the root command).
var errNotRoot = errors.New("not root")

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "x11droid",
		Short:         "Manage Waydroid instances in Podman on X11",
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Everything that touches podman/waydroid needs root. Refuse early
			// and cleanly rather than limping into a cryptic podman failure.
			// The read-only commands stay usable without root.
			switch cmd.Name() {
			case "version", "help", "status", "config":
				return nil
			}
			if !system.IsRoot() {
				fmt.Fprintln(os.Stderr, "error: x11droid must run as root — try: sudo x11droid")
				return errNotRoot
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return launchTUI()
		},
	}
	root.SetVersionTemplate("x11droid {{.Version}}\n")

	root.AddCommand(
		cmdList(),
		cmdSpawn(),
		cmdAttach(),
		cmdHide(),
		cmdStart(),
		cmdStop(),
		cmdRM(),
		cmdLogs(),
		cmdShell(),
		cmdAndroidShell(),
		cmdInstall(),
		cmdLogcat(),
		cmdConfig(),
		cmdPrune(),
		cmdSetup(),
		cmdVersion(),
	)

	return root
}

func launchTUI() error {
	sess := system.Detect()
	if w := sess.Warning(); w != "" {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	zone.NewGlobal() // clickable mouse zones for the TUI
	p := tea.NewProgram(tui.New(sess), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
