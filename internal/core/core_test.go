package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deckplane/deckplane-cli/internal/envfile"
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

func TestComposeModes(t *testing.T) {
	dir := t.TempDir()
	s := Settings{Port: 8080}.withDefaults()
	if err := writeCompose(dir, s); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(filepath.Join(dir, composeFile))
	for _, want := range []string{"registry.deckplane.com/deckplane/core:latest", `"8080:3000"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("port mode compose missing %q", want)
		}
	}
	if strings.Contains(string(out), "traefik") {
		t.Error("port mode compose mentions traefik")
	}

	s = Settings{Expose: ExposeDomain, Domain: "dp.example.com", TraefikNetwork: "proxy", Version: "v1.2.0"}.withDefaults()
	if err := writeCompose(dir, s); err != nil {
		t.Fatal(err)
	}
	out, _ = os.ReadFile(filepath.Join(dir, composeFile))
	for _, want := range []string{"Host(`dp.example.com`)", "name: proxy", "core:v1.2.0", "certresolver=le"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("domain mode compose missing %q", want)
		}
	}
	if strings.Contains(string(out), "ports:") {
		t.Error("domain mode compose publishes a port")
	}
}
