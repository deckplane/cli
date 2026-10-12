package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deckplane/cli/internal/envfile"
)

func TestValidateLicenseKey(t *testing.T) {
	for key, ok := range map[string]bool{
		"":                             false,
		"abc_123456789012345678":       false,
		"dpl_short":                    false,
		"dpl_abcdefghijklmnopqrstuvwx": true,
		"dpl_abcdefghij klmnopqrstuv":  false,
	} {
		if err := ValidateLicenseKey(key); (err == nil) != ok {
			t.Errorf("ValidateLicenseKey(%q) = %v", key, err)
		}
	}
}

func TestFillSecretsKeepsOldDatabasePassword(t *testing.T) {
	env := envfile.New("")
	env.Set("DATABASE_URL", "postgresql://deckplane:deckplane@postgres:5432/deckplane")
	env.Set("JWT_SECRET", "keep-me")
	fillSecrets(env)
	if env.Get("DB_PASSWORD") != "deckplane" {
		t.Fatalf("DB_PASSWORD = %q", env.Get("DB_PASSWORD"))
	}
	if env.Get("JWT_SECRET") != "keep-me" {
		t.Fatal("existing secret was replaced")
	}
	if len(env.Get("ENCRYPTION_KEY")) != 64 {
		t.Fatalf("ENCRYPTION_KEY length = %d", len(env.Get("ENCRYPTION_KEY")))
	}
}

func TestFillSecretsFreshInstall(t *testing.T) {
	env := envfile.New("")
	fillSecrets(env)
	pw := env.Get("DB_PASSWORD")
	if len(pw) != 48 || !strings.Contains(env.Get("DATABASE_URL"), ":"+pw+"@postgres") {
		t.Fatalf("DB_PASSWORD %q / DATABASE_URL %q", pw, env.Get("DATABASE_URL"))
	}
}

func composeFor(t *testing.T, s Settings) string {
	t.Helper()
	dir := t.TempDir()
	if err := writeCompose(dir, s.withDefaults()); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(filepath.Join(dir, composeFile))
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func expect(t *testing.T, mode, out string, want, unwanted []string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("%s compose missing %q", mode, w)
		}
	}
	for _, u := range unwanted {
		if strings.Contains(out, u) {
			t.Errorf("%s compose must not contain %q", mode, u)
		}
	}
}

// Core itself never publishes a port: only Traefik does.
func corePublishesNoPort(t *testing.T, out string) {
	t.Helper()
	core := out[strings.Index(out, "  core:"):]
	if i := strings.Index(core, "\n  traefik:"); i >= 0 {
		core = core[:i]
	}
	if strings.Contains(core, "ports:") {
		t.Error("the core service publishes a port")
	}
}

func TestComposeBundledWithoutDomain(t *testing.T) {
	out := composeFor(t, Settings{Proxy: ProxyBundled})
	expect(t, "bundled HTTP", out,
		[]string{"name: deckplane-core\n", "name: deckplane_pgdata", "name: deckplane_letsencrypt","registry.deckplane.com/deckplane/core:latest", "image: " + traefikImage, `"80:80"`,
			"PathPrefix(`/`)", "priority=1", "entrypoints=web", "name: deckplane-proxy"},
		[]string{`"443:443"`, "acme", "tls=true", "external: true"})
	corePublishesNoPort(t, out)
}

func TestComposeBundledWithDomain(t *testing.T) {
	out := composeFor(t, Settings{Domain: "dp.example.com", Proxy: ProxyBundled, ACMEEmail: "ops@example.com"})
	expect(t, "bundled HTTPS", out,
		[]string{"Host(`dp.example.com`)", "acme.email=ops@example.com", `"443:443"`, "redirections.entrypoint.to=websecure",
			"traefik.docker.network=deckplane-proxy", "entrypoints=websecure", "certresolver=le", "letsencrypt:"},
		[]string{"PathPrefix", "external: true"})
	corePublishesNoPort(t, out)
}

func TestComposeExistingTraefik(t *testing.T) {
	out := composeFor(t, Settings{Domain: "dp.example.com", Proxy: ProxyExisting, TraefikNetwork: "proxy", CertResolver: "myle", Entrypoint: "secure", Version: "v1.2.0"})
	expect(t, "existing HTTPS", out,
		[]string{"Host(`dp.example.com`)", "name: proxy", "external: true", "core:v1.2.0", "certresolver=myle", "entrypoints=secure"},
		[]string{"ports:", "deckplane-traefik"})

	out = composeFor(t, Settings{Proxy: ProxyExisting, TraefikNetwork: "proxy", HTTPEntrypoint: "http"})
	expect(t, "existing HTTP", out,
		[]string{"PathPrefix(`/`)", "entrypoints=http", "external: true"},
		[]string{"ports:", "deckplane-traefik", "tls=true"})
}

func TestOldPortInstallMovesBehindTraefik(t *testing.T) {
	env := envfile.New("")
	env.Set("DECKPLANE_EXPOSE", "port")
	env.Set("DECKPLANE_PORT", "3000")
	s := settingsFrom(env)
	if !s.Bundled() || s.HTTPS() {
		t.Fatalf("old port install: proxy %q domain %q", s.Proxy, s.Domain)
	}
	s.store(env)
	if _, ok := env.Lookup("DECKPLANE_PORT"); ok {
		t.Error("DECKPLANE_PORT was kept")
	}

	env = envfile.New("")
	env.Set("DECKPLANE_EXPOSE", "domain")
	env.Set("DECKPLANE_DOMAIN", "dp.example.com")
	env.Set("TRAEFIK_NETWORK", "proxy")
	if s := settingsFrom(env); s.Proxy != ProxyExisting || s.TraefikNetwork != "proxy" {
		t.Fatalf("old domain install: %+v", s)
	}
}

func TestWriteComposeMovesOldComposeFileAside(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(old, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeCompose(dir, Settings{Proxy: ProxyBundled}.withDefaults()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("docker-compose.yml is still next to compose.yaml")
	}
	if _, err := os.Stat(old + ".old"); err != nil {
		t.Error("the old file was not kept as docker-compose.yml.old")
	}
}
