// Package core installs and manages Deckplane Core (the self-hosted control
// plane) as a Docker Compose stack: Core itself plus its PostgreSQL database.
// Core never publishes a port of its own; it is always reached through
// Traefik, either one already running on the host or one the stack runs.
package core

import (
	"fmt"
	"strings"

	"github.com/deckplane/cli/internal/envfile"
	"github.com/deckplane/cli/internal/stack"
)

const (
	// DefaultRegistry hosts the private Core images. It authenticates
	// `docker login -u license -p <license key>` against Deckplane Cloud.
	DefaultRegistry = "registry.deckplane.com"
	imageRepo       = "deckplane/core"
	registryUser    = "license"

	DefaultLicenseAPI = "https://api.deckplane.com"

	// projectName must stay unique on the host: Docker Compose treats every
	// container of a project as its own, and `up --remove-orphans` deletes
	// the ones its file does not list. Deckplane Cloud runs as "deckplane".
	projectName = "deckplane-core"
	// legacyProject is the project name used up to v0.1.0.
	legacyProject = "deckplane"
	dbVolume      = "deckplane_pgdata"
	certVolume    = "deckplane_letsencrypt"
	containerName = "deckplane-core"
	traefikName   = "deckplane-traefik"
	// bundledNetwork is the proxy network of the Traefik the CLI runs itself;
	// other apps on this host can join it to be routed too.
	bundledNetwork = "deckplane-proxy"
	dbContainer    = "deckplane-postgres"
	serviceName    = "core"
	envFileName    = ".env"
	composeFile    = "compose.yaml"
)

// Proxy modes decide which Traefik routes to Core.
const (
	ProxyExisting = "existing" // a Traefik already running on this host
	ProxyBundled  = "bundled"  // a Traefik the CLI runs next to Core
)

// Settings are the install choices the CLI stores in .env (DECKPLANE_*
// keys) so later commands regenerate the same compose file. Without a
// Domain, Core answers plain HTTP on the server's address.
type Settings struct {
	Version        string
	Registry       string
	Domain         string
	Proxy          string
	TraefikNetwork string
	CertResolver   string
	Entrypoint     string // HTTPS entrypoint, used with a domain
	HTTPEntrypoint string // HTTP entrypoint, used without a domain
	ACMEEmail      string
}

func settingsFrom(env *envfile.File) Settings {
	s := Settings{
		Version:        env.Get("DECKPLANE_VERSION"),
		Registry:       env.Get("DECKPLANE_REGISTRY"),
		Domain:         env.Get("DECKPLANE_DOMAIN"),
		Proxy:          env.Get("DECKPLANE_PROXY"),
		TraefikNetwork: env.Get("TRAEFIK_NETWORK"),
		CertResolver:   env.Get("TRAEFIK_CERT_RESOLVER"),
		Entrypoint:     env.Get("TRAEFIK_ENTRYPOINT"),
		HTTPEntrypoint: env.Get("TRAEFIK_HTTP_ENTRYPOINT"),
		ACMEEmail:      env.Get("ACME_EMAIL"),
	}
	// v0.1.0 installs could publish a port (DECKPLANE_EXPOSE=port); they
	// move behind a Traefik the stack runs itself.
	if s.Proxy == "" && env.Get("DECKPLANE_EXPOSE") != "domain" {
		s.Proxy = ProxyBundled
	}
	return s.withDefaults()
}

func (s Settings) withDefaults() Settings {
	if s.Version == "" {
		s.Version = "latest"
	}
	if s.Registry == "" {
		s.Registry = DefaultRegistry
	}
	if s.Proxy == "" {
		s.Proxy = ProxyExisting
	}
	if s.CertResolver == "" {
		s.CertResolver = "le"
	}
	if s.Entrypoint == "" {
		s.Entrypoint = "websecure"
	}
	if s.HTTPEntrypoint == "" {
		s.HTTPEntrypoint = "web"
	}
	if s.Proxy == ProxyBundled {
		s.TraefikNetwork, s.CertResolver, s.Entrypoint, s.HTTPEntrypoint = bundledNetwork, "le", "websecure", "web"
	}
	return s
}

func (s Settings) store(env *envfile.File) {
	env.Set("DECKPLANE_VERSION", s.Version)
	env.Set("DECKPLANE_REGISTRY", s.Registry)
	env.Delete("DECKPLANE_EXPOSE")
	env.Delete("DECKPLANE_PORT")
	env.Set("DECKPLANE_DOMAIN", s.Domain)
	env.Set("DECKPLANE_PROXY", s.Proxy)
	env.Set("TRAEFIK_NETWORK", s.TraefikNetwork)
	env.Set("TRAEFIK_CERT_RESOLVER", s.CertResolver)
	env.Set("TRAEFIK_ENTRYPOINT", s.Entrypoint)
	env.Set("TRAEFIK_HTTP_ENTRYPOINT", s.HTTPEntrypoint)
	if s.Bundled() && s.Domain != "" {
		env.Set("ACME_EMAIL", s.ACMEEmail)
	}
}

// Bundled reports whether the CLI runs its own Traefik.
func (s Settings) Bundled() bool { return s.Proxy == ProxyBundled }

// HTTPS reports whether Core is served on a domain with a certificate.
func (s Settings) HTTPS() bool { return s.Domain != "" }

// Image is the full Core image reference.
func (s Settings) Image() string { return s.Registry + "/" + imageRepo + ":" + s.Version }

// URL is the address people open to reach Core.
func (s Settings) URL() string {
	if s.HTTPS() {
		return "https://" + s.Domain
	}
	host := stack.LocalIP()
	if host == "" {
		host = "localhost"
	}
	return "http://" + host
}

// ValidateLicenseKey checks the shape of a Deckplane Cloud license key.
// Whether the key is active is checked by the registry when signing in.
func ValidateLicenseKey(key string) error {
	switch {
	case key == "":
		return fmt.Errorf("the license key is empty")
	case !strings.HasPrefix(key, "dpl_"):
		return fmt.Errorf("license keys start with \"dpl_\" — copy the full key from Deckplane Cloud → Licenses")
	case len(key) < 20 || strings.ContainsAny(key, " \t\r\n"):
		return fmt.Errorf("this does not look like a complete license key — copy it again from Deckplane Cloud")
	}
	return nil
}

// fillSecrets adds Core's runtime configuration, generating secrets only when
// they are missing so re-running install never rotates them.
func fillSecrets(env *envfile.File) {
	if env.Get("DB_PASSWORD") == "" {
		// Installs made by the old CLI only have the password inside
		// DATABASE_URL; reuse it so the existing database keeps working.
		if pw := passwordFromURL(env.Get("DATABASE_URL")); pw != "" {
			env.Set("DB_PASSWORD", pw)
		} else {
			env.Set("DB_PASSWORD", stack.RandomHex(24))
		}
	}
	env.SetDefault("NODE_ENV", "production")
	env.SetDefault("PORT", "3000")
	env.SetDefault("HOST", "0.0.0.0")
	env.SetDefault("LOG_LEVEL", "info")
	env.SetDefault("DATABASE_URL", "postgresql://deckplane:"+env.Get("DB_PASSWORD")+"@postgres:5432/deckplane")
	env.SetDefault("JWT_SECRET", stack.RandomHex(32))
	env.SetDefault("AGENT_BOOTSTRAP_TOKEN", stack.RandomHex(16))
	env.SetDefault("ENCRYPTION_KEY", stack.RandomHex(32))
	env.SetDefault("LICENSE_API_URL", DefaultLicenseAPI)
}

func passwordFromURL(u string) string {
	_, rest, ok := strings.Cut(u, "://")
	if !ok {
		return ""
	}
	creds, _, ok := strings.Cut(rest, "@")
	if !ok {
		return ""
	}
	_, pw, _ := strings.Cut(creds, ":")
	return pw
}
