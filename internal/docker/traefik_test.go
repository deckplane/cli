package docker

import "testing"

func TestParseTraefikConfigFromFlags(t *testing.T) {
	resolver, entrypoint := parseTraefikConfig([]string{
		"--providers.docker=true",
		"--entrypoints.web.address=:80",
		"--entrypoints.https.address=:443",
		"--certificatesResolvers.myresolver.acme.email=a@b.c",
	}, nil)
	if resolver != "myresolver" || entrypoint != "https" {
		t.Fatalf("got resolver %q entrypoint %q", resolver, entrypoint)
	}
}

func TestParseTraefikConfigFromEnv(t *testing.T) {
	resolver, entrypoint := parseTraefikConfig(nil, []string{
		"TRAEFIK_CERTIFICATESRESOLVERS_LETSENCRYPT_ACME_EMAIL=a@b.c",
		"TRAEFIK_ENTRYPOINTS_WEBSECURE_ADDRESS=:443",
	})
	if resolver != "letsencrypt" || entrypoint != "websecure" {
		t.Fatalf("got resolver %q entrypoint %q", resolver, entrypoint)
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
	_, entrypoint := parseTraefikConfig([]string{"--entrypoints.web.address=:80", "--entrypoints.secure.address=:8443"}, nil)
	if entrypoint != "secure" {
		t.Fatalf("entrypoint = %q", entrypoint)
	}
	_, entrypoint = parseTraefikConfig([]string{"--entrypoints.a.address=:8443", "--entrypoints.b.address=:9443"}, nil)
	if entrypoint != "" {
		t.Fatalf("ambiguous entrypoints should not be guessed, got %q", entrypoint)
	}
}
