package docker

import (
	"strings"
	"testing"
)

func TestParseTraefikConfigFromFlags(t *testing.T) {
	cfg := parseTraefikConfig([]string{
		"--providers.docker=true",
		"--entrypoints.web.address=:80",
		"--entrypoints.https.address=:443",
		"--certificatesResolvers.myresolver.acme.email=a@b.c",
	}, nil)
	if cfg.resolver != "myresolver" || cfg.https != "https" || cfg.http != "web" || strings.Join(cfg.entrypoints, ",") != "https,web" {
		t.Fatalf("got %+v", cfg)
	}
}

func TestParseTraefikConfigFromEnv(t *testing.T) {
	cfg := parseTraefikConfig(nil, []string{
		"TRAEFIK_CERTIFICATESRESOLVERS_LETSENCRYPT_ACME_EMAIL=a@b.c",
		"TRAEFIK_ENTRYPOINTS_WEBSECURE_ADDRESS=:443",
	})
	if cfg.resolver != "letsencrypt" || cfg.https != "websecure" {
		t.Fatalf("got %+v", cfg)
	}
}

func TestLabelVotesFindStaticConfig(t *testing.T) {
	app := inspected{}
	app.Config.Labels = map[string]string{
		"traefik.http.routers.app.entrypoints":      "secure",
		"traefik.http.routers.app.tls.certresolver": "acme",
		"traefik.http.routers.plain.entrypoints":    "web", // no TLS: not a candidate
	}
	resolvers, entrypoints := labelVotes([]inspected{app})
	if mostCommon(resolvers) != "acme" || mostCommon(entrypoints) != "secure" {
		t.Fatalf("got %v %v", resolvers, entrypoints)
	}
}

func TestNetworkRankPrefersProxyNetworks(t *testing.T) {
	if !(networkRank("traefik-public") < networkRank("app_default") && networkRank("app_default") < networkRank("bridge")) {
		t.Fatal("unexpected network order")
	}
}

func TestParseTraefikConfigPicksOnlyNonHTTPEntrypoint(t *testing.T) {
	cfg := parseTraefikConfig([]string{"--entrypoints.web.address=:80", "--entrypoints.secure.address=:8443"}, nil)
	if cfg.https != "secure" || cfg.http != "web" {
		t.Fatalf("got %+v", cfg)
	}
	cfg = parseTraefikConfig([]string{"--entrypoints.a.address=:8443", "--entrypoints.b.address=:9443"}, nil)
	if cfg.https != "" {
		t.Fatalf("ambiguous entrypoints should not be guessed, got %q", cfg.https)
	}
}
