package cmd

import (
	"github.com/deckplane/cli/internal/agent"
	"github.com/deckplane/cli/internal/paths"
	"github.com/yigit433/kommando/v3"
)

func agentCmd() *kommando.Command {
	return &kommando.Command{
		Name:        "agent",
		Description: "Connect this Docker host to Deckplane Core",
		SubCommands: []*kommando.Command{
			agentInstallCmd(),
			agentAction("status", "Show whether the agent is running", nil, func(in *agent.Installation, _ *kommando.Context) error {
				return in.Status()
			}),
			agentAction("logs", "Show the agent's logs", []kommando.Flag{tailFlag, followFlag}, func(in *agent.Installation, ctx *kommando.Context) error {
				tail, err := integer(ctx, "tail")
				if err != nil {
					return err
				}
				return in.Logs(tail, boolean(ctx, "follow"))
			}),
			agentAction("update", "Update the agent to the latest (or a given) version",
				[]kommando.Flag{{Name: "version", Type: kommando.FlagString, Description: "version to switch to"}},
				func(in *agent.Installation, ctx *kommando.Context) error { return in.Update(str(ctx, "version")) }),
			agentAction("uninstall", "Remove the agent (keeps its identity unless --remove-data)",
				[]kommando.Flag{{Name: "remove-data", Type: kommando.FlagBool, Description: "also delete the agent's identity and settings"}},
				func(in *agent.Installation, ctx *kommando.Context) error {
					return in.Uninstall(boolean(ctx, "remove-data"))
				}),
		},
	}
}

func agentAction(name, desc string, flags []kommando.Flag, run func(*agent.Installation, *kommando.Context) error) *kommando.Command {
	return &kommando.Command{
		Name:        name,
		Description: desc,
		Flags:       append([]kommando.Flag{dirFlag(paths.AgentDir, "the agent")}, flags...),
		Execute: func(ctx *kommando.Context) error {
			in, err := agent.Open(dir(ctx, paths.AgentDir), boolean(ctx, "verbose"))
			if err != nil {
				return err
			}
			return run(in, ctx)
		},
	}
}

func agentInstallCmd() *kommando.Command {
	return &kommando.Command{
		Name:        "install",
		Description: "Install the agent (connects to a Core on this machine automatically)",
		Flags: []kommando.Flag{
			{Name: "server-url", Short: 's', Type: kommando.FlagString, Env: "DECKPLANE_SERVER_URL", Description: "Deckplane Core address, e.g. https://deckplane.example.com"},
			{Name: "token", Short: 't', Type: kommando.FlagString, Env: "DECKPLANE_AGENT_TOKEN", Description: "agent token (print it on the Core machine: deckplane core token)"},
			{Name: "name", Type: kommando.FlagString, Description: "name shown in Deckplane (default: this machine's hostname)"},
			{Name: "version", Type: kommando.FlagString, Description: "agent version to install (default latest)"},
			dirFlag(paths.AgentDir, "the agent"),
			yesFlag,
		},
		Execute: func(ctx *kommando.Context) error {
			return agent.Install(agent.InstallOptions{
				Dir:       dir(ctx, paths.AgentDir),
				ServerURL: str(ctx, "server-url"),
				Token:     str(ctx, "token"),
				Name:      str(ctx, "name"),
				Version:   str(ctx, "version"),
				Yes:       boolean(ctx, "yes"),
				Verbose:   boolean(ctx, "verbose"),
			})
		},
	}
}
