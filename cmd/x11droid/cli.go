package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/thereisnotime/x11droid/internal/config"
	"github.com/thereisnotime/x11droid/internal/container"
	"github.com/thereisnotime/x11droid/internal/kernel"
	"github.com/thereisnotime/x11droid/internal/version"
)

// twPrintf writes to a tabwriter, ignoring errors that surface through Flush.
func twPrintf(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}

func cmdList() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "ps"},
		Short:   "List all instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			instances, err := container.List()
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			twPrintf(w, "NAME\tID\tSTATUS\tIMAGE\n")
			for _, i := range instances {
				twPrintf(w, "%s\t%s\t%s\t%s\n", i.Name, i.ID, i.Status, i.Image)
			}
			return w.Flush()
		},
	}
}

func cmdSpawn() *cobra.Command {
	var gapps, hidearm, apps, noPV bool
	var deviceName string

	c := &cobra.Command{
		Use:   "spawn <name>",
		Short: "Create and start a new instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()
			w, h := cfg.EffectiveDims()
			return container.Spawn(container.SpawnOpts{
				Name:       args[0],
				DeviceName: deviceName,
				GApps:      gapps,
				HideARM:    hidearm,
				Apps:       apps,
				PV:         !noPV,
				Width:      w,
				Height:     h,
				Compositor: cfg.Compositor,
			})
		},
	}
	c.Flags().StringVar(&deviceName, "device-name", "", "Android device/model name (default: instance name)")
	c.Flags().BoolVar(&gapps, "gapps", false, "enable Google Play Store")
	c.Flags().BoolVar(&hidearm, "hidearm", false, "enable libndk ARM translation")
	c.Flags().BoolVar(&apps, "apps", false, "install F-Droid, Aurora, Obtainium, Shelter after first boot")
	c.Flags().BoolVar(&noPV, "no-pv", false, "disable persistent volume")
	return c
}

func cmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version, commit and build date",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			twPrintf(w, "version\t%s\n", version.Version)
			twPrintf(w, "commit\t%s\n", version.Commit)
			twPrintf(w, "built\t%s\n", version.Date)
			return w.Flush()
		},
	}
}

func cmdAttach() *cobra.Command {
	return &cobra.Command{
		Use:   "attach [name]",
		Short: "List running instances, or open the GUI for one",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				fmt.Printf("opening GUI for %s...\n", args[0])
				return container.ShowUI(args[0])
			}
			running, err := container.Running()
			if err != nil {
				return err
			}
			if len(running) == 0 {
				fmt.Println("no running instances — start one first, then: x11droid attach <name>")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			twPrintf(w, "NAME\tID\tSTATUS\n")
			for _, i := range running {
				twPrintf(w, "%s\t%s\t%s\n", i.Name, i.ID, i.Status)
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Println("\nopen one with: x11droid attach <name>")
			return nil
		},
	}
}

func cmdStart() *cobra.Command {
	return &cobra.Command{
		Use:   "start <name>",
		Short: "Start a stopped instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return container.Start(args[0])
		},
	}
}

func cmdStop() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <name>",
		Short: "Stop a running instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return container.Stop(args[0])
		},
	}
}

func cmdRM() *cobra.Command {
	var purge bool
	c := &cobra.Command{
		Use:     "rm <name>",
		Aliases: []string{"remove", "delete"},
		Short:   "Remove an instance (optionally its persisted data too)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if purge {
				return container.Purge(args[0])
			}
			return container.Remove(args[0])
		},
	}
	c.Flags().BoolVar(&purge, "purge", false, "also delete the instance's persisted Android data (~3GB)")
	return c
}

func cmdLogs() *cobra.Command {
	return &cobra.Command{
		Use:   "logs <name>",
		Short: "Show instance logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := container.Logs(args[0])
			if err != nil {
				return err
			}
			fmt.Print(out)
			return nil
		},
	}
}

func cmdShell() *cobra.Command {
	return &cobra.Command{
		Use:   "shell <name>",
		Short: "Open a bash shell inside an instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := exec.Command("podman", "exec", "-it", args[0], "bash")
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		},
	}
}

func cmdSetup() *cobra.Command {
	setup := &cobra.Command{
		Use:   "setup",
		Short: "System setup and status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdSetupStatus().RunE(cmd, args)
		},
	}
	setup.AddCommand(
		cmdSetupStatus(),
		cmdSetupLoad(),
		cmdSetupUnload(),
		cmdSetupBuild(),
	)
	return setup
}

func cmdSetupStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show setup status",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

			twPrintf(w, "COMPONENT\tSTATUS\n")

			podmanOK := "ok"
			if !container.PodmanInstalled() {
				podmanOK = "missing"
			}
			twPrintf(w, "podman\t%s\n", podmanOK)

			for _, mod := range kernel.Status() {
				state := "missing"
				switch mod.State {
				case kernel.StateLoaded:
					state = "loaded"
				case kernel.StateBuiltIn, kernel.StateOptional:
					state = "built-in"
				}
				twPrintf(w, "%s\t%s\n", mod.Name, state)
			}

			imageState := "not built"
			if container.ImageExists("x11droid:latest") {
				imageState = "ok"
			}
			twPrintf(w, "x11droid:latest\t%s\n", imageState)

			return w.Flush()
		},
	}
}

func cmdSetupLoad() *cobra.Command {
	return &cobra.Command{
		Use:   "load",
		Short: "Load kernel modules (binder_linux)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return kernel.Load()
		},
	}
}

func cmdSetupUnload() *cobra.Command {
	return &cobra.Command{
		Use:   "unload",
		Short: "Unload kernel modules",
		RunE: func(cmd *cobra.Command, args []string) error {
			return kernel.Unload()
		},
	}
}

func cmdSetupBuild() *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Build the x11droid container image",
		RunE: func(cmd *cobra.Command, args []string) error {
			buildCmd, err := container.BuildImageCmd()
			if err != nil {
				return err
			}
			buildCmd.Stdout = os.Stdout
			buildCmd.Stderr = os.Stderr
			return buildCmd.Run()
		},
	}
}
