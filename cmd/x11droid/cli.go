package main

import (
	"fmt"
	"os"
	"os/exec"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/thereisnotime/x11droid/internal/container"
	"github.com/thereisnotime/x11droid/internal/kernel"
)

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
			fmt.Fprintln(w, "NAME\tID\tSTATUS\tIMAGE")
			for _, i := range instances {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", i.Name, i.ID, i.Status, i.Image)
			}
			return w.Flush()
		},
	}
}

func cmdSpawn() *cobra.Command {
	var gapps, hidearm, noPV bool

	c := &cobra.Command{
		Use:   "spawn <name>",
		Short: "Create and start a new instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return container.Spawn(container.SpawnOpts{
				Name:    args[0],
				GApps:   gapps,
				HideARM: hidearm,
				PV:      !noPV,
			})
		},
	}
	c.Flags().BoolVar(&gapps, "gapps", false, "enable Google Play Store")
	c.Flags().BoolVar(&hidearm, "hidearm", false, "enable libndk ARM translation")
	c.Flags().BoolVar(&noPV, "no-pv", false, "disable persistent volume")
	return c
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
	return &cobra.Command{
		Use:     "rm <name>",
		Aliases: []string{"remove", "delete"},
		Short:   "Remove an instance",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return container.Remove(args[0])
		},
	}
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
		cmdSetupAuth(),
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

			fmt.Fprintln(w, "COMPONENT\tSTATUS")

			podmanOK := "ok"
			if !container.PodmanInstalled() {
				podmanOK = "missing"
			}
			fmt.Fprintf(w, "podman\t%s\n", podmanOK)

			for _, mod := range kernel.Status() {
				state := "missing"
				switch mod.State {
				case kernel.StateLoaded:
					state = "loaded"
				case kernel.StateBuiltIn, kernel.StateOptional:
					state = "built-in"
				}
				fmt.Fprintf(w, "%s\t%s\n", mod.Name, state)
			}

			imageState := "not built"
			if container.ImageExists("x11droid:latest") {
				imageState = "ok"
			}
			fmt.Fprintf(w, "x11droid:latest\t%s\n", imageState)

			return w.Flush()
		},
	}
}

func cmdSetupAuth() *cobra.Command {
	return &cobra.Command{
		Use:   "auth",
		Short: "Authenticate sudo (cache credentials)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := container.SudoAuthCmd()
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
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
