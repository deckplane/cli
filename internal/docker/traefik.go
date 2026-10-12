package docker

import (
	"encoding/json"
	"os/exec"
	"sort"
	"strings"
)

// Traefik describes a Traefik reverse proxy already running on this host,
// as far as it can be read from its container and from other containers'
// routing labels. Empty fields mean "could not tell".
type Traefik struct {
	Container      string
	Networks       []string
	CertResolver   string
	Entrypoint     string // the HTTPS entrypoint, e.g. "websecure"
	HTTPEntrypoint string // the plain HTTP entrypoint on :80, e.g. "web"
}

type inspected struct {
	Name   string
	Args   []string
	Config struct {
		Image  string
		Cmd    []string
		Env    []string
		Labels map[string]string
	}
	NetworkSettings struct {
		Networks map[string]json.RawMessage
	}
}

func inspectRunning() []inspected {
	ids, err := exec.Command("docker", "ps", "-q").Output()
	if err != nil || len(strings.TrimSpace(string(ids))) == 0 {
		return nil
	}
	out, err := exec.Command("docker", append([]string{"inspect"}, strings.Fields(string(ids))...)...).Output()
	if err != nil {
		return nil
	}
	var containers []inspected
	if json.Unmarshal(out, &containers) != nil {
		return nil
	}
	return containers
}

// FindTraefik returns the running Traefik containers, ignoring the one the
// CLI runs itself (skip).
func FindTraefik(skip string) []Traefik {
	containers := inspectRunning()
	resolverVotes, entrypointVotes := labelVotes(containers)

	var found []Traefik
	for _, c := range containers {
		name := strings.TrimPrefix(c.Name, "/")
		if name == skip || !strings.Contains(strings.ToLower(c.Config.Image), "traefik") {
			continue
		}
		t := Traefik{Container: name}
		for n := range c.NetworkSettings.Networks {
			if n != "host" && n != "none" {
				t.Networks = append(t.Networks, n)
			}
		}
		sort.Slice(t.Networks, func(i, j int) bool { return networkRank(t.Networks[i]) < networkRank(t.Networks[j]) })
		args := append(append([]string{}, c.Config.Cmd...), c.Args...)
		t.CertResolver, t.Entrypoint, t.HTTPEntrypoint = parseTraefikConfig(args, c.Config.Env)
		if t.CertResolver == "" {
			t.CertResolver = mostCommon(resolverVotes)
		}
		if t.Entrypoint == "" {
			t.Entrypoint = mostCommon(entrypointVotes)
		}
		found = append(found, t)
	}
	return found
}

// networkRank puts networks that look like a proxy network first.
func networkRank(n string) int {
	l := strings.ToLower(n)
	switch {
	case strings.Contains(l, "traefik"), strings.Contains(l, "proxy"):
		return 0
	case n == "bridge":
		return 2
	}
	return 1
}

// parseTraefikConfig reads the certificate resolver name and the HTTPS and
// HTTP entrypoints from Traefik's command-line flags or environment. The
// HTTPS entrypoint is the one on :443, or else the only one not on :80
// (Traefik often listens on another port inside its container).
func parseTraefikConfig(args, env []string) (resolver, entrypoint, httpEntrypoint string) {
	addresses := map[string]string{}
	for _, a := range args {
		a = strings.TrimLeft(a, "-")
		const resolverPrefix = "certificatesresolvers."
		if resolver == "" && strings.HasPrefix(strings.ToLower(a), resolverPrefix) {
			resolver, _, _ = strings.Cut(a[len(resolverPrefix):], ".")
		}
		if rest, ok := strings.CutPrefix(a, "entrypoints."); ok {
			name, setting, _ := strings.Cut(rest, ".")
			if value, ok := strings.CutPrefix(strings.ToLower(setting), "address="); ok {
				addresses[name] = value
			}
		}
	}
	for _, e := range env {
		key, value, _ := strings.Cut(e, "=")
		if rest, ok := strings.CutPrefix(key, "TRAEFIK_CERTIFICATESRESOLVERS_"); ok && resolver == "" {
			name, _, _ := strings.Cut(rest, "_")
			resolver = strings.ToLower(name)
		}
		if rest, ok := strings.CutPrefix(key, "TRAEFIK_ENTRYPOINTS_"); ok && strings.HasSuffix(rest, "_ADDRESS") {
			addresses[strings.ToLower(strings.TrimSuffix(rest, "_ADDRESS"))] = value
		}
	}

	var others []string
	for name, address := range addresses {
		switch {
		case strings.HasSuffix(address, ":443"):
			entrypoint = name
		case strings.HasSuffix(address, ":80"):
			httpEntrypoint = name
		default:
			others = append(others, name)
		}
	}
	if entrypoint == "" && len(others) == 1 {
		entrypoint = others[0]
	}
	return resolver, entrypoint, httpEntrypoint
}

// labelVotes counts the resolvers and TLS entrypoints other containers route
// through, which reveals settings kept in Traefik's static config file.
func labelVotes(containers []inspected) (resolvers, entrypoints map[string]int) {
	resolvers, entrypoints = map[string]int{}, map[string]int{}
	for _, c := range containers {
		for k, v := range c.Config.Labels {
			if !strings.HasPrefix(k, "traefik.http.routers.") || v == "" {
				continue
			}
			switch {
			case strings.HasSuffix(k, ".tls.certresolver"):
				resolvers[v]++
			case strings.HasSuffix(k, ".entrypoints") && !strings.Contains(v, ","):
				router := strings.TrimSuffix(k, ".entrypoints")
				if c.Config.Labels[router+".tls"] == "true" || c.Config.Labels[router+".tls.certresolver"] != "" {
					entrypoints[v]++
				}
			}
		}
	}
	return resolvers, entrypoints
}

func mostCommon(votes map[string]int) string {
	best, bestCount := "", 0
	for v, n := range votes {
		if n > bestCount || (n == bestCount && v < best) {
			best, bestCount = v, n
		}
	}
	return best
}
