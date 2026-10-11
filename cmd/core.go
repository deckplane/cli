package cmd

import (
	"fmt"

	"github.com/deckplane/deckplane-cli/internal/core"
	"github.com/deckplane/deckplane-cli/internal/paths"
	"github.com/deckplane/deckplane-cli/internal/ui"
	"github.com/yigit433/kommando/v3"
)

func coreCmd() *kommando.Command {
	return &kommando.Command{
		Name:        "core",
		Description: "Install and manage Deckplane Core on this machine",
		SubCommands: []*kommando.Command{
			coreInstallCmd(),
			coreAction("status", "Show whether Deckplane Core is running", (*core.Installation).Status),
			coreLogsCmd(),
			coreAction("start", "Start Deckplane Core", (*core.Installation).Start),
			coreAction("stop", "Stop Deckplane Core (nothing is deleted)", (*core.Installation).Stop),
			coreAction("restart", "Restart Deckplane Core and apply changed settings", (*core.Installation).Restart),
			coreUpdateCmd(),
			coreAction("setup", "Create the first admin account", (*core.Installation).Setup),
			coreTokenCmd(),
			coreConfigCmd(),
			coreLicenseCmd(),
			coreUninstallCmd(),
		},
	}
}

// openCore loads the install selected by --dir.
func openCore(ctx *kommando.Context) (*core.Installation, error) {
	return core.Open(dir(ctx, paths.CoreDir), boolean(ctx, "verbose"))
}

func coreAction(name, desc string, run func(*core.Installation) error) *kommando.Command {
	return &kommando.Command{
		Name:        name,
		Description: desc,
		Flags:       []kommando.Flag{dirFlag(paths.CoreDir, "Core")},
		Execute: func(ctx *kommando.Context) error {
			in, err := openCore(ctx)
			if err != nil {
				return err
			}
			return run(in)
		},
	}
}

func coreInstallCmd() *kommando.Command {
	return &kommando.Command{
		Name:        "install",
		Description: "Install Deckplane Core (asks for anything you do not pass)",
		Flags: []kommando.Flag{
			{Name: "license-key", Short: 'k', Type: kommando.FlagString, Env: "DECKPLANE_LICENSE_KEY", Description: "license key from Deckplane Cloud → Licenses (dpl_...)"},
			{Name: "licence-key", Type: kommando.FlagString, Description: "same as --license-key"},
			{Name: "port", Short: 'p', Type: kommando.FlagInt, Description: "port to open Core on (default 3000)"},
			{Name: "domain", Type: kommando.FlagString, Description: "serve Core on this domain over HTTPS (uses the Traefik on this machine, or runs one)"},
			{Name: "acme-email", Type: kommando.FlagString, Description: "email for Let's Encrypt when Deckplane runs its own Traefik"},
			{Name: "traefik-network", Type: kommando.FlagString, Description: "use the Traefik on this Docker network (default: detected)"},
			{Name: "cert-resolver", Type: kommando.FlagString, Description: "certificate resolver of your Traefik (default: detected, or le)"},
			{Name: "traefik-entrypoint", Type: kommando.FlagString, Description: "HTTPS entrypoint of your Traefik (default: detected, or websecure)"},
			{Name: "version", Type: kommando.FlagString, Description: "Core version to install (default latest)"},
			{Name: "admin-email", Type: kommando.FlagString, Description: "create the first admin with this email (password from DECKPLANE_ADMIN_PASSWORD)"},
			{Name: "admin-name", Type: kommando.FlagString, Description: "full name of the first admin"},
			{Name: "registry", Type: kommando.FlagString, Env: "DECKPLANE_REGISTRY", Description: "image registry (default " + core.DefaultRegistry + ")"},
			{Name: "license-api-url", Type: kommando.FlagString, Env: "DECKPLANE_LICENSE_API_URL", Description: "Deckplane Cloud API (default " + core.DefaultLicenseAPI + ")"},
			dirFlag(paths.CoreDir, "Core"),
			yesFlag,
		},
		Execute: func(ctx *kommando.Context) error {
			port, err := integer(ctx, "port")
			if err != nil {
				return err
			}
			key := str(ctx, "license-key")
			if key == "" {
				key = str(ctx, "licence-key")
			}
			return core.Install(core.InstallOptions{
				Dir:            dir(ctx, paths.CoreDir),
				LicenseKey:     key,
				LicenseAPIURL:  str(ctx, "license-api-url"),
				Registry:       str(ctx, "registry"),
				Version:        str(ctx, "version"),
				Port:           port,
				Domain:         str(ctx, "domain"),
				TraefikNetwork: str(ctx, "traefik-network"),
				CertResolver:   str(ctx, "cert-resolver"),
				Entrypoint:     str(ctx, "traefik-entrypoint"),
				ACMEEmail:      str(ctx, "acme-email"),
				AdminEmail:     str(ctx, "admin-email"),
				AdminName:      str(ctx, "admin-name"),
				Yes:            boolean(ctx, "yes"),
				Verbose:        boolean(ctx, "verbose"),
			})
		},
	}
}

func coreLogsCmd() *kommando.Command {
	return &kommando.Command{
		Name:        "logs",
		Description: "Show Deckplane Core logs",
		Flags:       []kommando.Flag{dirFlag(paths.CoreDir, "Core"), tailFlag, followFlag},
		Execute: func(ctx *kommando.Context) error {
			in, err := openCore(ctx)
			if err != nil {
				return err
			}
			tail, err := integer(ctx, "tail")
			if err != nil {
				return err
			}
			return in.Logs(tail, boolean(ctx, "follow"))
		},
	}
}

func coreUpdateCmd() *kommando.Command {
	return &kommando.Command{
		Name:        "update",
		Description: "Update Deckplane Core to the latest (or a given) version",
		Flags: []kommando.Flag{
			dirFlag(paths.CoreDir, "Core"),
			{Name: "version", Type: kommando.FlagString, Description: "version to switch to (default: keep the current channel)"},
		},
		Execute: func(ctx *kommando.Context) error {
			in, err := openCore(ctx)
			if err != nil {
				return err
			}
			return in.Update(str(ctx, "version"))
		},
	}
}

func coreTokenCmd() *kommando.Command {
	return &kommando.Command{
		Name:        "token",
		Description: "Print the token agents use to connect to this Core",
		Flags:       []kommando.Flag{dirFlag(paths.CoreDir, "Core")},
		Execute: func(ctx *kommando.Context) error {
			in, err := openCore(ctx)
			if err != nil {
				return err
			}
			fmt.Fprintln(ctx.Output(), in.AgentToken())
			return nil
		},
	}
}

func coreConfigCmd() *kommando.Command {
	return &kommando.Command{
		Name:        "config",
		Description: "Show or change Core settings",
		Flags: []kommando.Flag{
			dirFlag(paths.CoreDir, "Core"),
			{Name: "reveal", Type: kommando.FlagBool, Description: "show secrets in full"},
		},
		Execute: func(ctx *kommando.Context) error {
			in, err := openCore(ctx)
			if err != nil {
				return err
			}
			in.ShowConfig(boolean(ctx, "reveal"))
			return nil
		},
		SubCommands: []*kommando.Command{
			{
				Name:        "get",
				Description: "Print one setting",
				Flags:       []kommando.Flag{dirFlag(paths.CoreDir, "Core")},
				Execute: func(ctx *kommando.Context) error {
					if len(ctx.Args()) != 1 {
						return ui.NewError("Expected one setting name", "Usage:  deckplane core config get KEY", nil)
					}
					in, err := openCore(ctx)
					if err != nil {
						return err
					}
					v, ok := in.Env.Lookup(ctx.Args()[0])
					if !ok {
						return fmt.Errorf("%s is not set", ctx.Args()[0])
					}
					fmt.Fprintln(ctx.Output(), v)
					return nil
				},
			},
			{
				Name:        "set",
				Description: "Change settings: set KEY=VALUE [KEY=VALUE ...]",
				Flags: []kommando.Flag{
					dirFlag(paths.CoreDir, "Core"),
					{Name: "restart", Type: kommando.FlagBool, Description: "restart Core so the change takes effect"},
				},
				Execute: func(ctx *kommando.Context) error {
					in, err := openCore(ctx)
					if err != nil {
						return err
					}
					return in.SetConfig(ctx.Args(), boolean(ctx, "restart"))
				},
			},
		},
	}
}

func coreLicenseCmd() *kommando.Command {
	return &kommando.Command{
		Name:        "license",
		Description: "Replace the license key: license <dpl_...>",
		Aliases:     []string{"licence"},
		Flags:       []kommando.Flag{dirFlag(paths.CoreDir, "Core")},
		Execute: func(ctx *kommando.Context) error {
			if len(ctx.Args()) != 1 {
				return ui.NewError("Expected the new license key", "Usage:  deckplane core license dpl_...", nil)
			}
			in, err := openCore(ctx)
			if err != nil {
				return err
			}
			return in.SetLicense(ctx.Args()[0])
		},
	}
}

func coreUninstallCmd() *kommando.Command {
	return &kommando.Command{
		Name:        "uninstall",
		Description: "Remove Deckplane Core (keeps data unless --remove-data)",
		Flags: []kommando.Flag{
			dirFlag(paths.CoreDir, "Core"),
			{Name: "remove-data", Type: kommando.FlagBool, Description: "also delete the database and all settings"},
			yesFlag,
		},
		Execute: func(ctx *kommando.Context) error {
			in, err := openCore(ctx)
			if err != nil {
				return err
			}
			return in.Uninstall(boolean(ctx, "remove-data"), boolean(ctx, "yes"))
		},
	}
}
