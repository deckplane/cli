package core

import (
	"errors"
	"fmt"
	"net"
	"net/mail"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/deckplane/deckplane-cli/internal/docker"
	"github.com/deckplane/deckplane-cli/internal/envfile"
	"github.com/deckplane/deckplane-cli/internal/paths"
	"github.com/deckplane/deckplane-cli/internal/stack"
	"github.com/deckplane/deckplane-cli/internal/ui"
)

// InstallOptions are the flags of `deckplane core install`. Zero values mean
// "keep the existing value, or ask, or use the default".
type InstallOptions struct {
	Dir            string
	LicenseKey     string
	LicenseAPIURL  string
	Registry       string
	Version        string
	Port           int
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
	if s.Expose == ExposeDomain {
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
	if err := pullAndStart(p); err != nil {
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
	if s.Bundled() {
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
// existing install, then (on a fresh install) the user's answers.
func applyAccessChoices(s *Settings, o InstallOptions, existing, interactive bool) error {
	if o.Version != "" {
		s.Version = o.Version
	}
	if o.Registry != "" {
		s.Registry = o.Registry
	}
	switch {
	case o.Domain != "":
		s.Expose, s.Domain = ExposeDomain, o.Domain
		if err := chooseProxy(s, o, interactive); err != nil {
			return err
		}
	case o.Port != 0:
		s.Expose, s.Port = ExposePort, o.Port
	case !existing && interactive:
		if err := askAccess(s, o); err != nil {
			return err
		}
	}
	*s = s.withDefaults()
	return checkAccess(*s)
}

func askAccess(s *Settings, o InstallOptions) error {
	ui.Println("")
	def := 1
	if len(docker.FindTraefik(traefikName)) > 0 {
		def = 0
	}
	choice, err := ui.Choose("How will people reach Deckplane Core?", []string{
		"With my own domain over HTTPS (e.g. https://deckplane.example.com)",
		"Through a port on this machine (no domain needed, e.g. http://server-ip:3000)",
	}, def)
	if err != nil {
		return err
	}
	if choice == 1 {
		s.Expose = ExposePort
		answer, err := ui.AskRequired("Port:", strconv.Itoa(s.Port), validatePort)
		if err != nil {
			return err
		}
		s.Port, _ = strconv.Atoi(answer)
		return nil
	}

	s.Expose = ExposeDomain
	if s.Domain, err = ui.AskRequired("Domain (e.g. deckplane.example.com):", s.Domain, validateDomain); err != nil {
		return err
	}
	if addrs, err := net.LookupHost(s.Domain); err != nil || len(addrs) == 0 {
		ui.Warn("%s does not resolve yet. Point its DNS A record to this server; HTTPS starts working once it does.", s.Domain)
	} else {
		ui.Info("%s points to %s", s.Domain, strings.Join(addrs, ", "))
	}
	return chooseProxy(s, o, true)
}

// chooseProxy decides which Traefik serves the domain: one already running
// on this host (found automatically), or one the CLI runs next to Core.
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
	} else {
		ui.Info("No Traefik is running on this machine.")
	}

	s.Proxy = ProxyBundled
	if s.ACMEEmail != "" {
		return nil
	}
	if !interactive {
		return ui.NewError("An email address is needed for the HTTPS certificate",
			"Deckplane will run its own Traefik with free Let's Encrypt certificates.\n"+
				"Pass the email Let's Encrypt should use:  --acme-email you@example.com\n"+
				"To use a Traefik you already run, pass --traefik-network instead.", nil)
	}
	ui.Println("Deckplane will run its own Traefik with free HTTPS certificates from Let's Encrypt.")
	ui.Info("Ports 80 and 443 must be free and reachable from the internet.")
	email, err := ui.AskRequired("Email for certificate expiry notices:", "", validateEmail)
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
	if o.CertResolver == "" {
		switch {
		case t.CertResolver != "":
			s.CertResolver = t.CertResolver
		case interactive:
			if s.CertResolver, err = ui.AskRequired("Traefik certificate resolver name:", "le", nil); err != nil {
				return err
			}
		}
	}
	if o.Entrypoint == "" {
		switch {
		case t.Entrypoint != "":
			s.Entrypoint = t.Entrypoint
		case interactive:
			if s.Entrypoint, err = ui.AskRequired("Traefik HTTPS entrypoint name:", "websecure", nil); err != nil {
				return err
			}
		}
	}
	ui.Info("Network %s · certificate resolver %s · entrypoint %s", s.TraefikNetwork, s.CertResolver, s.Entrypoint)
	return nil
}

// checkAccess catches problems that would only show up after starting.
func checkAccess(s Settings) error {
	switch {
	case s.Bundled():
		if docker.ContainerState(traefikName) != "missing" {
			return nil // our Traefik already holds the ports
		}
		for _, port := range []int{80, 443} {
			if !stack.PortFree(port) {
				return ui.NewError(fmt.Sprintf("Port %d is already in use", port),
					"HTTPS needs ports 80 and 443, and another web server (nginx, Apache, another proxy…) is using one.\n"+
						"If it is Traefik, run the installer again and choose it. Otherwise stop it,\n"+
						"or serve Deckplane on a port instead:  deckplane core install --port 3000", nil)
			}
		}
	case s.Expose == ExposeDomain:
		if s.TraefikNetwork == "bridge" {
			return ui.NewError("Traefik only uses Docker's default bridge network",
				"Attach Traefik to a named network (docker network create proxy), then pass --traefik-network proxy.", nil)
		}
		if !docker.NetworkExists(s.TraefikNetwork) {
			return ui.NewError(fmt.Sprintf("Docker network %q does not exist", s.TraefikNetwork),
				"Pass the network your Traefik uses with --traefik-network,\nor leave it out and Deckplane runs its own Traefik.", nil)
		}
	default:
		running := docker.ContainerState(containerName) != "missing"
		if !running && !stack.PortFree(s.Port) {
			return ui.NewError(fmt.Sprintf("Port %d is already in use", s.Port),
				"Choose another port, for example:  deckplane core install --port 8080", nil)
		}
	}
	return nil
}

func validatePort(v string) error {
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 65535 {
		return errors.New("enter a number between 1 and 65535")
	}
	return nil
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

func pullAndStart(p docker.Project) error {
	ui.Step("Downloading Deckplane Core (the first time can take a few minutes)")
	pullArgs := []string{"pull"}
	if devMode() {
		pullArgs = append(pullArgs, "--ignore-pull-failures")
	}
	if err := p.Stream(pullArgs...); err != nil {
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
