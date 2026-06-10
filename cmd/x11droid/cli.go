package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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
	var gapps, hidearm, fdroid, aurora, obtainium, shelter, devOptions, root, noPV bool
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
				FDroid:     fdroid,
				Aurora:     aurora,
				Obtainium:  obtainium,
				Shelter:    shelter,
				DevOptions: devOptions,
				Root:       root,
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
	c.Flags().BoolVar(&fdroid, "fdroid", false, "install F-Droid after first boot")
	c.Flags().BoolVar(&aurora, "aurora", false, "install Aurora Store after first boot")
	c.Flags().BoolVar(&obtainium, "obtainium", false, "install Obtainium after first boot")
	c.Flags().BoolVar(&shelter, "shelter", false, "install Shelter after first boot")
	c.Flags().BoolVar(&devOptions, "dev-options", false, "enable Android Developer Options on first boot")
	c.Flags().BoolVar(&root, "root", false, "install Magisk (root) on first boot")
	c.Flags().BoolVar(&noPV, "no-pv", false, "disable persistent volume")
	return c
}

func cmdPrune() *cobra.Command {
	var all bool
	c := &cobra.Command{
		Use:   "prune",
		Short: "Show instance disk usage and delete leftover (orphan) data",
		RunE: func(cmd *cobra.Command, args []string) error {
			dds, err := container.DataDirs()
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			twPrintf(w, "INSTANCE\tSIZE\tDATA\n")
			for _, d := range dds {
				state := "container"
				if !d.HasContainer {
					state = "orphan"
				}
				twPrintf(w, "%s\t%s\t%s\n", d.Name, d.Size, state)
			}
			_ = w.Flush()

			if all {
				instances, _ := container.List()
				for _, i := range instances {
					if err := container.Purge(i.Name); err != nil {
						fmt.Fprintln(os.Stderr, err)
					}
				}
			}
			removed, err := container.PruneOrphans()
			if err != nil {
				return err
			}
			if len(removed) == 0 {
				fmt.Println("\nnothing to prune")
			} else {
				fmt.Printf("\nremoved data for: %s\n", strings.Join(removed, ", "))
			}
			return nil
		},
	}
	c.Flags().BoolVar(&all, "all", false, "remove ALL instances and their data, not just orphans")
	return c
}

func cmdConfig() *cobra.Command {
	var width, height int
	var orientation, compositor string
	c := &cobra.Command{
		Use:   "config",
		Short: "Show or set instance defaults (resolution, orientation, compositor)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()
			if cmd.Flags().Changed("width") {
				cfg.Width = width
			}
			if cmd.Flags().Changed("height") {
				cfg.Height = height
			}
			if cmd.Flags().Changed("orientation") {
				cfg.Orientation = orientation
			}
			if cmd.Flags().Changed("compositor") {
				cfg.Compositor = compositor
			}
			if cmd.Flags().NFlag() > 0 {
				if err := cfg.Save(); err != nil {
					return err
				}
				cfg = config.Load() // re-load so normalized values show
			}
			ew, eh := cfg.EffectiveDims()
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			twPrintf(w, "resolution\t%dx%d\n", cfg.Width, cfg.Height)
			twPrintf(w, "orientation\t%s\n", cfg.Orientation)
			twPrintf(w, "compositor\t%s\n", cfg.Compositor)
			twPrintf(w, "window\t%dx%d\n", ew, eh)
			return w.Flush()
		},
	}
	c.Flags().IntVar(&width, "width", 0, "portrait width")
	c.Flags().IntVar(&height, "height", 0, "portrait height")
	c.Flags().StringVar(&orientation, "orientation", "", "portrait|landscape")
	c.Flags().StringVar(&compositor, "compositor", "", "auto|weston|cage")
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

func cmdHide() *cobra.Command {
	return &cobra.Command{
		Use:   "hide <name>",
		Short: "Close the GUI window but keep the instance running",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("hiding GUI for %s (still running — reopen with: x11droid attach %s)...\n", args[0], args[0])
			return container.HideUI(args[0])
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
