package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deckplane/cli/internal/docker"
	"github.com/deckplane/cli/internal/envfile"
	"github.com/deckplane/cli/internal/stack"
	"github.com/deckplane/cli/internal/ui"
)

// Installation is an existing Core install on this machine.
type Installation struct {
	Dir     string
	Env     *envfile.File
	Project docker.Project
}

// Open loads the install in dir, or explains how to create one.
func Open(dir string, verbose bool) (*Installation, error) {
	env, err := envfile.Load(filepath.Join(dir, envFileName))
	if os.IsNotExist(err) {
		return nil, ui.NewError("Deckplane Core is not installed in "+dir,
			"Install it with:  deckplane core install\nIf it lives in another folder, pass --dir.", nil)
	}
	if os.IsPermission(err) {
		return nil, permissionError(dir, err)
	}
	if err != nil {
		return nil, err
	}
	return &Installation{Dir: dir, Env: env, Project: docker.Project{Dir: dir, Verbose: verbose}}, nil
}

func (in *Installation) Settings() Settings { return settingsFrom(in.Env) }

// Status prints what is running and where.
func (in *Installation) Status() error {
	s := in.Settings()
	state := docker.ContainerState(containerName)
	ui.Title("Deckplane Core")
	ui.Println("  Status:    %s", colorState(state))
	ui.Println("  Database:  %s", colorState(docker.ContainerState(dbContainer)))
	ui.Println("  Address:   %s", s.URL())
	switch {
	case s.Bundled() && s.HTTPS():
		ui.Println("  Proxy:     %s (Deckplane's Traefik, Let's Encrypt for %s)", colorState(docker.ContainerState(traefikName)), s.ACMEEmail)
	case s.Bundled():
		ui.Println("  Proxy:     %s (Deckplane's Traefik)", colorState(docker.ContainerState(traefikName)))
	default:
		ui.Println("  Proxy:     your Traefik on network %s", s.TraefikNetwork)
	}
	if img := docker.ContainerImage(containerName); img != "" {
		ui.Println("  Image:     %s", img)
	}
	ui.Println("  License:   %s", stack.Mask(in.Env.Get("LICENSE_KEY")))
	ui.Println("  Files:     %s", in.Dir)
	ui.Println("")
	if state != "healthy" && state != "starting" {
		ui.Info("Start it with `deckplane core start`, or see why with `deckplane core logs`.")
	}
	return nil
}

func colorState(state string) string {
	switch state {
	case "healthy", "running":
		return ui.Green(state)
	case "starting":
		return ui.Yellow(state)
	case "missing":
		return ui.Red("not created")
	}
	return ui.Red(state)
}

// Logs shows Core's logs, optionally following them.
func (in *Installation) Logs(tail int, follow bool) error {
	args := []string{"logs", "--tail", fmt.Sprint(tail)}
	if follow {
		args = append(args, "--follow")
	}
	return in.Project.Attach(append(args, serviceName)...)
}

// ensureImage downloads the Core image when it is not on this machine, so
// `up` never pulls from the private registry without signing in.
func (in *Installation) ensureImage() error {
	s := in.Settings()
	if devMode() || docker.ImageExists(s.Image()) {
		return nil
	}
	if err := signIn(s.Registry, in.Env.Get("LICENSE_KEY")); err != nil {
		return err
	}
	defer docker.Logout(s.Registry)
	return pullImages(in.Project, s)
}

// Start brings the stack up.
func (in *Installation) Start() error {
	if err := docker.Check(); err != nil {
		return err
	}
	if err := in.ensureImage(); err != nil {
		return err
	}
	ui.Step("Starting Deckplane Core")
	if err := releaseLegacyContainers(); err != nil {
		return err
	}
	if err := in.Project.Run("up", "--detach"); err != nil {
		return err
	}
	if err := stack.WaitHealthy(containerName, 3*time.Minute); err != nil {
		return err
	}
	ui.Success("Deckplane Core is running at %s", in.Settings().URL())
	return nil
}

// Stop stops the containers without removing anything.
func (in *Installation) Stop() error {
	ui.Step("Stopping Deckplane Core")
	if err := releaseLegacyContainers(); err != nil {
		return err
	}
	if err := in.Project.Run("stop"); err != nil {
		return err
	}
	ui.Success("Stopped. Start it again with `deckplane core start`.")
	return nil
}

// Restart recreates the Core container so .env changes take effect.
func (in *Installation) Restart() error {
	if err := in.ensureImage(); err != nil {
		return err
	}
	ui.Step("Restarting Deckplane Core")
	if err := releaseLegacyContainers(); err != nil {
		return err
	}
	if err := in.Project.Run("up", "--detach", "--force-recreate", serviceName); err != nil {
		return err
	}
	if err := stack.WaitHealthy(containerName, 3*time.Minute); err != nil {
		return err
	}
	ui.Success("Deckplane Core restarted")
	return nil
}

// Update pulls a newer image (or the given version) and restarts.
func (in *Installation) Update(version string) error {
	if err := docker.Check(); err != nil {
		return err
	}
	s := in.Settings()
	if version != "" {
		s.Version = version
	}
	// Installs from v0.1.0 move behind a Traefik here, which needs port 80.
	if err := checkAccess(s); err != nil {
		return err
	}
	s.store(in.Env)
	fillSecrets(in.Env)
	if err := in.Env.Save(); err != nil {
		return err
	}
	if err := writeCompose(in.Dir, s); err != nil {
		return err
	}
	if err := signIn(s.Registry, in.Env.Get("LICENSE_KEY")); err != nil {
		return err
	}
	defer docker.Logout(s.Registry)
	before := docker.ContainerImage(containerName)
	if err := pullAndStart(in.Project, s); err != nil {
		return err
	}
	ui.Success("Deckplane Core is up to date (%s)", s.Image())
	if before != "" && before != s.Image() {
		ui.Info("Previously running: %s", before)
	}
	return nil
}

// Uninstall removes the containers. With removeData it also deletes the
// database volume and the install folder.
func (in *Installation) Uninstall(removeData, yes bool) error {
	if removeData && !yes {
		ui.Warn("This permanently deletes the Deckplane Core database and %s.", in.Dir)
		if !ui.Interactive() {
			return ui.NewError("Refusing to delete data without confirmation", "Add --yes to confirm.", nil)
		}
		answer, err := ui.Ask("Type \"delete\" to confirm:", "")
		if err != nil {
			return err
		}
		if answer != "delete" {
			ui.Println("Cancelled. Nothing was removed.")
			return nil
		}
	}
	ui.Step("Removing Deckplane Core containers")
	if err := releaseLegacyContainers(); err != nil {
		return err
	}
	args := []string{"down", "--remove-orphans"}
	if removeData {
		args = append(args, "--volumes")
	}
	if err := in.Project.Run(args...); err != nil {
		return err
	}
	if removeData {
		if err := os.RemoveAll(in.Dir); err != nil {
			return fmt.Errorf("could not remove %s: %w", in.Dir, err)
		}
		ui.Success("Deckplane Core and all its data were removed")
		return nil
	}
	ui.Success("Deckplane Core was removed. Your data is kept in %s and the database volume.", in.Dir)
	ui.Info("Reinstall any time with `deckplane core install`; add --remove-data to delete everything.")
	return nil
}

// ShowConfig prints the settings with secrets masked.
func (in *Installation) ShowConfig(reveal bool) {
	for _, k := range in.Env.Keys() {
		v := in.Env.Get(k)
		if !reveal && stack.IsSecretKey(k) {
			v = stack.Mask(v)
		}
		ui.Println("%s=%s", k, v)
	}
}

// SetConfig applies KEY=VALUE pairs (an empty value removes nothing; it sets
// an empty string) and optionally restarts Core.
func (in *Installation) SetConfig(pairs []string, restart bool) error {
	if len(pairs) == 0 {
		return ui.NewError("Nothing to set", "Usage:  deckplane core config set KEY=VALUE [KEY=VALUE ...]", nil)
	}
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" || strings.ContainsAny(k, " \t") {
			return ui.NewError(fmt.Sprintf("%q is not KEY=VALUE", p), "Example:  deckplane core config set LOG_LEVEL=debug", nil)
		}
		in.Env.Set(k, v)
	}
	if err := in.Env.Save(); err != nil {
		return err
	}
	ui.Success("Saved %d setting(s)", len(pairs))
	if restart {
		return in.Restart()
	}
	ui.Info("Apply the change with `deckplane core restart`.")
	return nil
}

// SetLicense replaces the license key after checking it with the registry.
func (in *Installation) SetLicense(key string) error {
	if err := ValidateLicenseKey(key); err != nil {
		return ui.NewError("Invalid license key", err.Error(), nil)
	}
	registry := in.Settings().Registry
	if err := signIn(registry, key); err != nil {
		return err
	}
	docker.Logout(registry)
	in.Env.Set("LICENSE_KEY", key)
	if err := in.Env.Save(); err != nil {
		return err
	}
	ui.Success("License key updated")
	return in.Restart()
}

// AgentToken returns the token agents use to register with this Core.
func (in *Installation) AgentToken() string { return in.Env.Get("AGENT_BOOTSTRAP_TOKEN") }

// Setup creates the first admin account interactively.
func (in *Installation) Setup() error {
	required, err := setupRequired(in.Project)
	if err != nil {
		return ui.NewError("Could not reach Deckplane Core", "Is it running? Check with `deckplane core status`.", err)
	}
	if !required {
		ui.Success("An admin account already exists — sign in at %s", in.Settings().URL())
		return nil
	}
	return SetupAdmin(in.Project)
}
