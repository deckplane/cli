// Package cmd defines the deckplane command tree with Kommando.
package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/deckplane/cli/internal/buildinfo"
	"github.com/deckplane/cli/internal/doctor"
	"github.com/yigit433/kommando/v3"
)

// NewApp builds the deckplane CLI.
func NewApp() *kommando.App {
	app := kommando.New("deckplane",
		kommando.WithDescription("Install and manage Deckplane on your own servers."),
		kommando.WithOutput(os.Stdout),
		kommando.WithGlobalFlags(
			kommando.Flag{Name: "verbose", Short: 'v', Type: kommando.FlagBool, Description: "show full Docker output"},
		),
	)
	for _, c := range []*kommando.Command{coreCmd(), agentCmd(), doctorCmd(), versionCmd()} {
		if err := app.AddCommand(c); err != nil {
			panic(err)
		}
	}
	return app
}

func doctorCmd() *kommando.Command {
	return &kommando.Command{
		Name:        "doctor",
		Description: "Check that this machine is ready to run Deckplane",
		Execute:     func(*kommando.Context) error { return doctor.Run() },
	}
}

func versionCmd() *kommando.Command {
	return &kommando.Command{
		Name:        "version",
		Description: "Print the CLI version",
		Execute: func(ctx *kommando.Context) error {
			fmt.Fprintf(ctx.Output(), "deckplane %s (commit %s, built %s)\n", buildinfo.Version, buildinfo.Commit, buildinfo.Date)
			return nil
		},
	}
}

// ─── flag helpers ────────────────────────────────────────────────────────────

func str(ctx *kommando.Context, name string) string {
	v, _ := ctx.String(name)
	return v
}

func boolean(ctx *kommando.Context, name string) bool {
	v, _ := ctx.Bool(name)
	return v
}

func integer(ctx *kommando.Context, name string) (int, error) {
	v, ok := ctx.String(name)
	if !ok || v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("--%s must be a number", name)
	}
	return n, nil
}

func dirFlag(def func() string, what string) kommando.Flag {
	return kommando.Flag{Name: "dir", Short: 'd', Type: kommando.FlagString,
		Description: "install folder of " + what + " (default: " + def() + ")"}
}

func dir(ctx *kommando.Context, def func() string) string {
	if v := str(ctx, "dir"); v != "" {
		return v
	}
	return def()
}

var (
	yesFlag    = kommando.Flag{Name: "yes", Short: 'y', Type: kommando.FlagBool, Description: "do not ask questions; use flags and defaults"}
	tailFlag   = kommando.Flag{Name: "tail", Short: 'n', Type: kommando.FlagInt, Default: "200", Description: "number of recent lines to show"}
	followFlag = kommando.Flag{Name: "follow", Short: 'f', Type: kommando.FlagBool, Description: "keep streaming new log lines"}
)
