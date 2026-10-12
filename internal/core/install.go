package core

import (
	"errors"
	"fmt"
	"net"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deckplane/cli/internal/docker"
	"github.com/deckplane/cli/internal/envfile"
	"github.com/deckplane/cli/internal/paths"
	"github.com/deckplane/cli/internal/stack"
	"github.com/deckplane/cli/internal/ui"
)

// InstallOptions are the flags of `deckplane core install`. Zero values mean
// "keep the existing value, or ask, or use the default".
type InstallOptions struct {
	Dir            string
	LicenseKey     string
	LicenseAPIURL  string
	Registry       string
	Version        string
	Domain         string
	TraefikNetwork string
	CertResolver   string
	Entrypoint     string
	ACMEEmail      string
	AdminEmail     string
	AdminName      string
	AdminPassword  string
	Yes            bool // never ask questions; fail if something is missing
	Verbose        bool
}

// Install sets up Deckplane Core on this machine. Running it again is safe:
// existing secrets and data are kept, and newer settings are applied.
func Install(o InstallOptions) error {
	interactive := ui.Interactive() && !o.Yes
	ui.Title("Deckplane Core installer")

	ui.Step("Checking Docker")
	if err := docker.Check(); err != nil {
		return err
	}
	if err := docker.CheckCompose(); err != nil {
		return err
	}
	ui.Success("Docker %s with Compose is ready", docker.Version())

	if err := paths.EnsureWritable(o.Dir); err != nil {
		return permissionError(o.Dir, err)
	}
	envPath := filepath.Join(o.Dir, envFileName)
	env, err := envfile.Load(envPath)
	existing := err == nil
	switch {
	case existing:
		ui.Info("Found an existing installation in %s — its data and secrets are kept.", o.Dir)
	case os.IsNotExist(err):
		env = envfile.New(envPath)
		env.Comment("Deckplane Core settings, written by the deckplane CLI.")
		env.Comment("Change values with `deckplane core config set KEY=VALUE`.")
	default:
		return fmt.Errorf("could not read %s: %w", envPath, err)
	}

	key, err := resolveLicenseKey(o.LicenseKey, env.Get("LICENSE_KEY"), interactive)
	if err != nil {
		return err
	}

	s := settingsFrom(env)
	if err := applyAccessChoices(&s, o, existing, interactive); err != nil {
		return err
	}

	if err := signIn(s.Registry, key); err != nil {
		return err
	}
	defer docker.Logout(s.Registry)

	env.Set("LICENSE_KEY", key)
	if o.LicenseAPIURL != "" {
		env.Set("LICENSE_API_URL", o.LicenseAPIURL)
	}
	s.store(env)
	fillSecrets(env)
	if s.HTTPS() {
		env.SetDefault("PUBLIC_BASE_URL", "https://"+s.Domain)
	}
	if err := env.Save(); err != nil {
		return fmt.Errorf("could not save %s: %w", envPath, err)
	}
	if err := writeCompose(o.Dir, s); err != nil {
		return fmt.Errorf("could not write the compose file: %w", err)
	}
	ui.Success("Configuration saved in %s", o.Dir)

	p := docker.Project{Dir: o.Dir, Verbose: o.Verbose}
	if err := pullAndStart(p, s); err != nil {
		return err
	}

	adminCreated, err := offerAdminSetup(p, o, interactive)
	if err != nil {
		ui.Warn("Could not create the admin account: %v", err)
		ui.Info("Try again later with: deckplane core setup")
	}

	lines := []string{
		ui.Green("Deckplane Core is running"),
		"",
		"Address:  " + ui.Bold(s.URL()),
		"Files:    " + o.Dir,
		"",
	}
	if s.Bundled() && s.HTTPS() {
		lines = append(lines, "HTTPS certificate: requested automatically once DNS points here", "")
	}
	if !adminCreated {
		lines = append(lines, "Next: create your admin account → deckplane core setup")
	}
	lines = append(lines,
		"Connect a server → deckplane agent install",
		"Status and logs  → deckplane core status / logs",
	)
	ui.Box(lines...)
	return nil
}

func resolveLicenseKey(flagKey, storedKey string, interactive bool) (string, error) {
	key := strings.TrimSpace(flagKey)
	if key == "" {
		key = storedKey
	}
	if key == "" {
		if !interactive {
			return "", ui.NewError("A license key is required",
				"Pass it with:  deckplane core install --license-key dpl_...\n"+
					"Create one in Deckplane Cloud → Licenses (https://cloud.deckplane.com).", nil)
		}
		ui.Println("")
		ui.Println("Deckplane Core needs a license key. Create one in Deckplane Cloud → Licenses")
		ui.Println("(%s); it starts with %s and is shown only once.", ui.Cyan("https://cloud.deckplane.com"), ui.Bold("dpl_"))
		var err error
		key, err = ui.AskSecret("Paste your license key (hidden):", ValidateLicenseKey)
		if err != nil {
			return "", err
		}
	}
	if err := ValidateLicenseKey(key); err != nil {
		return "", ui.NewError("Invalid license key", err.Error(), nil)
	}
	return key, nil
}

// applyAccessChoices fills in how Core is reached: flags win, then the
// existing install, then (on a fresh install) the user's answers. Core is
// always served by Traefik; only which Traefik and whether there is a domain
// change.
func applyAccessChoices(s *Settings, o InstallOptions, existing, interactive bool) error {
	if o.Version != "" {
		s.Version = o.Version
	}
	if o.Registry != "" {
		s.Registry = o.Registry
	}
	switch {
	case o.Domain != "":
		s.Domain = o.Domain
	case !existing && interactive:
		if err := askDomain(s); err != nil {
			return err
		}
	}
	if !existing || o.Domain != "" || o.TraefikNetwork != "" {
		if err := chooseProxy(s, o, interactive); err != nil {
			return err
		}
	}
	*s = s.withDefaults()
	return checkAccess(*s)
}

func askDomain(s *Settings) error {
	ui.Println("")
	choice, err := ui.Choose("How will people open Deckplane Core?", []string{
		"With my own domain over HTTPS (recommended, e.g. https://deckplane.example.com)",
		"With this server's IP address over HTTP (no domain needed)",
	}, 0)
	if err != nil || choice == 1 {
		return err
	}
	if s.Domain, err = ui.AskRequired("Domain (e.g. deckplane.example.com):", s.Domain, validateDomain); err != nil {
		return err
	}
	if addrs, err := net.LookupHost(s.Domain); err != nil || len(addrs) == 0 {
		ui.Warn("%s does not resolve yet. Point its DNS A record to this server; HTTPS starts working once it does.", s.Domain)
	} else {
		ui.Info("%s points to %s", s.Domain, strings.Join(addrs, ", "))
	}
	return nil
}

// chooseProxy decides which Traefik routes to Core: one already running on
// this host (found automatically), or one the CLI runs next to Core.
func chooseProxy(s *Settings, o InstallOptions, interactive bool) error {
	if o.CertResolver != "" {
		s.CertResolver = o.CertResolver
	}
	if o.Entrypoint != "" {
		s.Entrypoint = o.Entrypoint
	}
	if o.ACMEEmail != "" {
		s.ACMEEmail = o.ACMEEmail
	}
	if o.TraefikNetwork != "" {
		s.Proxy, s.TraefikNetwork = ProxyExisting, o.TraefikNetwork
		return nil
	}

	if found := docker.FindTraefik(traefikName); len(found) > 0 {
		t := found[0]
		ui.Success("Found Traefik running in container %s", t.Container)
		use := true
		if interactive {
			var err error
			if use, err = ui.Confirm("Route Deckplane through this Traefik?", true); err != nil {
				return err
			}
		}
		if use {
			return useTraefik(s, t, o, interactive)
		}
	} else if docker.ContainerState(traefikName) == "missing" {
		ui.Info("No Traefik is running on this machine — Deckplane will run its own.")
	}

	s.Proxy = ProxyBundled
	if !s.HTTPS() {
		ui.Info("Port 80 must be free.")
		return nil
	}
	ui.Info("Ports 80 and 443 must be free and reachable from the internet.")
	if s.ACMEEmail != "" {
		return nil
	}
	if !interactive {
		return ui.NewError("An email address is needed for the HTTPS certificate",
			"Deckplane will run its own Traefik with free Let's Encrypt certificates.\n"+
				"Pass the email Let's Encrypt should use:  --acme-email you@example.com\n"+
				"To use a Traefik you already run, pass --traefik-network instead.", nil)
	}
	email, err := ui.AskRequired("Email for certificate expiry notices (Let's Encrypt):", "", validateEmail)
	s.ACMEEmail = email
	return err
}

// useTraefik routes Core through a detected Traefik, asking only for what
// could not be read from it.
func useTraefik(s *Settings, t docker.Traefik, o InstallOptions, interactive bool) error {
	s.Proxy = ProxyExisting
	switch {
	case len(t.Networks) == 0:
		return ui.NewError(fmt.Sprintf("Traefik (%s) is not attached to a Docker network", t.Container),
			"Pass the network to use with --traefik-network.", nil)
	case len(t.Networks) > 1 && interactive:
		i, err := ui.Choose("Which of Traefik's Docker networks should Deckplane join?", t.Networks, 0)
		if err != nil {
			return err
		}
		s.TraefikNetwork = t.Networks[i]
	default:
		s.TraefikNetwork = t.Networks[0]
	}

	var err error
	if !s.HTTPS() {
		if s.HTTPEntrypoint, err = detectedOrAsk(t.HTTPEntrypoint, "Traefik HTTP entrypoint name (port 80):", "web", interactive); err != nil {
			return err
		}
		ui.Info("Network %s · entrypoint %s", s.TraefikNetwork, s.HTTPEntrypoint)
		return nil
	}
	if o.CertResolver == "" {
		if s.CertResolver, err = detectedOrAsk(t.CertResolver, "Traefik certificate resolver name:", "le", interactive); err != nil {
			return err
		}
	}
	if o.Entrypoint == "" {
		if s.Entrypoint, err = detectedOrAsk(t.Entrypoint, "Traefik HTTPS entrypoint name:", "websecure", interactive); err != nil {
			return err
		}
	}
	ui.Info("Network %s · certificate resolver %s · entrypoint %s", s.TraefikNetwork, s.CertResolver, s.Entrypoint)
	return nil
}

// detectedOrAsk returns what was read from Traefik, or asks with def as the
// suggested answer (def without a terminal).
func detectedOrAsk(detected, label, def string, interactive bool) (string, error) {
	if detected != "" || !interactive {
		return firstNonEmpty(detected, def), nil
	}
	return ui.AskRequired(label, def, nil)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// checkAccess catches problems that would only show up after starting.
func checkAccess(s Settings) error {
	if s.Bundled() {
		if state := docker.ContainerState(traefikName); state == "running" || state == "healthy" {
			return nil // our Traefik already holds the ports
		}
		ports := []int{80}
		if s.HTTPS() {
			ports = append(ports, 443)
		}
		for _, port := range ports {
			if !stack.PortFree(port) {
				return portInUse(port)
			}
		}
		return nil
	}
	if s.TraefikNetwork == "bridge" {
		return ui.NewError("Traefik only uses Docker's default bridge network",
			"Attach Traefik to a named network (docker network create proxy), then pass --traefik-network proxy.", nil)
	}
	if !docker.NetworkExists(s.TraefikNetwork) {
		return ui.NewError(fmt.Sprintf("Docker network %q does not exist", s.TraefikNetwork),
			"Pass the network your Traefik uses with --traefik-network,\nor leave it out and Deckplane runs its own Traefik.", nil)
	}
	return nil
}

func portInUse(port int) error {
	hint := fmt.Sprintf("Deckplane Core is served through Traefik, and the Traefik Deckplane runs needs port %d.\n", port)
	if user := stack.PortUser(port); user != "" {
		hint += "It is used by " + user + ".\n"
	}
	hint += "If that is a Traefik running in Docker, use it:  --traefik-network <its network>\n" +
		"Otherwise stop that program (for example:  systemctl stop nginx) and run the installer again."
	return ui.NewError(fmt.Sprintf("Port %d is already in use", port), hint, nil)
}

func validateDomain(v string) error {
	if v == "" || strings.Contains(v, "://") || strings.ContainsAny(v, "/ ") || !strings.Contains(v, ".") {
		return errors.New("enter only the domain name, like deckplane.example.com")
	}
	return nil
}

func validateEmail(v string) error {
	if _, err := mail.ParseAddress(v); err != nil || !strings.Contains(v, "@") {
		return errors.New("enter a valid email address")
	}
	return nil
}

// signIn checks the license by signing in to the image registry with it.
// Setting DECKPLANE_DEV=1 skips this for locally built images.
func signIn(registry, key string) error {
	if devMode() {
		ui.Warn("DECKPLANE_DEV is set — skipping registry sign-in")
		return nil
	}
	ui.Step("Checking your license with %s", registry)
	err := docker.Login(registry, registryUser, key)
	if errors.Is(err, docker.ErrUnauthorized) {
		return ui.NewError("Your license key was not accepted",
			"Check in Deckplane Cloud → Licenses that the key is active, not expired or revoked,\n"+
				"and that you copied all of it.", nil)
	}
	if err != nil {
		return ui.NewError("Could not reach the Deckplane registry",
			"Check this machine's internet connection and firewall, then try again.\nRun `deckplane doctor` for details.", err)
	}
	ui.Success("License accepted")
	return nil
}

func devMode() bool { return os.Getenv("DECKPLANE_DEV") == "1" }

func pullAndStart(p docker.Project, s Settings) error {
	ui.Step("Downloading Deckplane Core (the first time can take a few minutes)")
	pullArgs := []string{"pull"}
	if devMode() {
		pullArgs = append(pullArgs, "--ignore-pull-failures")
	}
	if err := p.Stream(pullArgs...); err != nil {
		if !devMode() && docker.ImageMissing(s.Image()) {
			return ui.NewError(fmt.Sprintf("Deckplane Core %s is not published yet", s.Version),
				fmt.Sprintf("%s does not exist in the registry. Try again later,\nor install a published version with --version vX.Y.Z.", s.Image()), nil)
		}
		return ui.NewError("Downloading the images failed", "Run `deckplane doctor` to check the connection.", err)
	}
	ui.Step("Starting Deckplane Core")
	if err := p.Run("up", "--detach", "--remove-orphans"); err != nil {
		return err
	}
	ui.Step("Waiting for Deckplane Core to become ready")
	if err := stack.WaitHealthy(containerName, 3*time.Minute); err != nil {
		return err
	}
	ui.Success("Deckplane Core is healthy")
	return nil
}

func permissionError(dir string, err error) error {
	hint := "Choose another folder with --dir."
	if !paths.IsRoot() && strings.HasPrefix(dir, "/var/") {
		hint = "Run the command with sudo, for example:  sudo deckplane core install\n" + hint
	}
	return ui.NewError(fmt.Sprintf("Cannot write to %s", dir), hint, err)
}
