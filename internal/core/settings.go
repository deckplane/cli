// Package core installs and manages Deckplane Core (the self-hosted control
// plane) as a Docker Compose stack: Core itself plus its PostgreSQL database.
package core

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/deckplane/deckplane-cli/internal/envfile"
	"github.com/deckplane/deckplane-cli/internal/stack"
)

const (
	// DefaultRegistry hosts the private Core images. It authenticates
	// `docker login -u license -p <license key>` against Deckplane Cloud.
	DefaultRegistry = "registry.deckplane.com"
	imageRepo       = "deckplane/core"
	registryUser    = "license"

	DefaultLicenseAPI = "https://api.deckplane.com"
	DefaultPort       = 3000

	containerName = "deckplane-core"
	dbContainer   = "deckplane-postgres"
	serviceName   = "core"
	envFileName   = ".env"
	composeFile   = "compose.yaml"
)

// Expose modes decide how people reach Core.
const (
	ExposePort   = "port"   // publish a port on this machine
	ExposeDomain = "domain" // route a domain through an existing Traefik
)

// Settings are the install choices the CLI stores in .env (DECKPLANE_*
// keys) so later commands regenerate the same compose file.
type Settings struct {
	Version        string
	Registry       string
	Expose         string
	Port           int
	Domain         string
	TraefikNetwork string
	CertResolver   string
}

func settingsFrom(env *envfile.File) Settings {
	s := Settings{
		Version:        env.Get("DECKPLANE_VERSION"),
		Registry:       env.Get("DECKPLANE_REGISTRY"),
		Expose:         env.Get("DECKPLANE_EXPOSE"),
		Domain:         env.Get("DECKPLANE_DOMAIN"),
		TraefikNetwork: env.Get("TRAEFIK_NETWORK"),
		CertResolver:   env.Get("TRAEFIK_CERT_RESOLVER"),
	}
	s.Port, _ = strconv.Atoi(env.Get("DECKPLANE_PORT"))
	return s.withDefaults()
}

func (s Settings) withDefaults() Settings {
	if s.Version == "" {
		s.Version = "latest"
	}
	if s.Registry == "" {
		s.Registry = DefaultRegistry
	}
	if s.Expose == "" {
		s.Expose = ExposePort
	}
	if s.Port == 0 {
		s.Port = DefaultPort
	}
	if s.CertResolver == "" {
		s.CertResolver = "le"
	}
	return s
}

func (s Settings) store(env *envfile.File) {
	env.Set("DECKPLANE_VERSION", s.Version)
	env.Set("DECKPLANE_REGISTRY", s.Registry)
	env.Set("DECKPLANE_EXPOSE", s.Expose)
	env.Set("DECKPLANE_PORT", strconv.Itoa(s.Port))
	if s.Expose == ExposeDomain {
		env.Set("DECKPLANE_DOMAIN", s.Domain)
		env.Set("TRAEFIK_NETWORK", s.TraefikNetwork)
		env.Set("TRAEFIK_CERT_RESOLVER", s.CertResolver)
	}
}

// Image is the full Core image reference.
func (s Settings) Image() string { return s.Registry + "/" + imageRepo + ":" + s.Version }

// URL is the address people open to reach Core.
func (s Settings) URL() string {
	if s.Expose == ExposeDomain {
		return "https://" + s.Domain
	}
	host := stack.LocalIP()
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%d", host, s.Port)
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
