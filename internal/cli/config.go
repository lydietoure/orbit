package cli

import (
	"fmt"
	"strconv"

	"github.com/lydietoure/orbit/internal/app"
	"github.com/spf13/cobra"
)

// getCmdConfig builds `orbit config` and its subcommands. Today it
// only carries the scratchpad/dock settings; future scalar prefs
// (default editor, etc.) will hang off the same tree.
//
// The command path here uses `scratchpad` (what users think of
// when configuring this) while the thing being configured is
// conceptually "the dock" (where scratchpads live). Help text and
// the ORBIT_DOCK env var reflect the latter.
func getCmdConfig() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and update orbit settings",
	}
	cmd.AddCommand(newConfigScratchpadCmd())
	return cmd
}

func newConfigScratchpadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scratchpad",
		Short: "Configure the dock — where scratchpads live, and how they're provisioned",
	}
	cmd.AddCommand(
		newConfigScratchpadGetRootCmd(),
		newConfigScratchpadSetRootCmd(),
		newConfigScratchpadUnsetRootCmd(),
		newConfigScratchpadAutoCreateCmd(),
	)
	return cmd
}

func newConfigScratchpadGetRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get-root",
		Short: "Show the current dock root and auto-create setting",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, source, err := app.GetDockRoot(cmd.Context())
			if err != nil {
				return err
			}
			autoCreate, err := app.GetDockAutoCreate(cmd.Context())
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if source == app.DockRootUnset {
				fmt.Fprintf(out, "Dock root:   (unset)\n")
			} else {
				fmt.Fprintf(out, "Dock root:   %s (source: %s)\n", root, source)
			}
			fmt.Fprintf(out, "Auto-create: %t\n", autoCreate)
			return nil
		},
	}
}

func newConfigScratchpadSetRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-root <path>",
		Short: "Set the dock root — the directory where scratchpads live",
		Long: `Set the dock root.

The path is absolutized at set time so subsequent reads are stable
regardless of the working directory. The ORBIT_DOCK environment
variable, if set, still overrides this value at read time.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			abs, err := app.SetDockRoot(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Dock root set to %s\n", abs)
			return nil
		},
	}
}

func newConfigScratchpadUnsetRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unset-root",
		Short: "Clear the dock root",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := app.UnsetDockRoot(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Dock root cleared.")
			return nil
		},
	}
}

func newConfigScratchpadAutoCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "auto-create <true|false>",
		Short: "Toggle automatic scratchpad provisioning under the dock root",
		Long: `Toggle automatic scratchpad provisioning.

When true, ` + "`orbit work new`" + ` will create a scratchpad
subdirectory under the dock root for each new entry. When false
(default), scratchpad paths must be passed explicitly via -s /
--scratchpad. Accepts the usual truthy/falsy forms: true, false,
1, 0, yes, no, on, off (case-insensitive).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// strconv.ParseBool covers 1/0/t/f/T/F/true/false/TRUE/
			// FALSE/True/False; add the human-friendly aliases on
			// top so `auto-create yes` and `auto-create on` work too.
			v, err := parseBoolFlexible(args[0])
			if err != nil {
				return fmt.Errorf("invalid bool %q: accepts true/false, 1/0, yes/no, on/off", args[0])
			}
			if err := app.SetDockAutoCreate(cmd.Context(), v); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Auto-create set to %t\n", v)
			return nil
		},
	}
}

// parseBoolFlexible accepts the strconv.ParseBool set plus the
// common shell-friendly aliases (yes/no, on/off). Case-insensitive.
func parseBoolFlexible(s string) (bool, error) {
	switch s {
	case "yes", "Yes", "YES", "on", "On", "ON":
		return true, nil
	case "no", "No", "NO", "off", "Off", "OFF":
		return false, nil
	}
	return strconv.ParseBool(s)
}
