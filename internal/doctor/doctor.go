// Package doctor checks whether this machine is ready to run Deckplane and
// explains how to fix anything that is not.
package doctor

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/deckplane/cli/internal/core"
	"github.com/deckplane/cli/internal/docker"
	"github.com/deckplane/cli/internal/envfile"
	"github.com/deckplane/cli/internal/paths"
	"github.com/deckplane/cli/internal/ui"
)

type check struct {
	name string
	run  func() (detail string, err error)
}

// Run executes all checks and returns an error if any failed.
func Run() error {
	ui.Title("Deckplane doctor")
	checks := []check{
		{"Operating system", func() (string, error) { return runtime.GOOS + "/" + runtime.GOARCH, nil }},
		{"Docker", func() (string, error) { return docker.Version(), docker.Check() }},
		{"Docker Compose", func() (string, error) { return "", docker.CheckCompose() }},
		{"Deckplane Cloud", func() (string, error) { return reach(licenseAPI() + "/health") }},
		{"Deckplane registry", func() (string, error) { return reach("https://" + registry() + "/v2/") }},
		{"Traefik", traefikState},
		{"Deckplane Core", func() (string, error) { return installState(paths.CoreDir(), "deckplane-core") }},
		{"Deckplane agent", func() (string, error) { return installState(paths.AgentDir(), "deckplane-agent") }},
	}
	failed := 0
	for _, c := range checks {
		detail, err := c.run()
		switch {
		case err != nil:
			failed++
			ui.Println("%s %-20s %s", ui.Red("✗"), c.name, err.Error())
			var hinted *ui.Error
			if errors.As(err, &hinted) && hinted.Hint != "" {
				ui.Info("%s", hinted.Hint)
			}
		default:
			ui.Println("%s %-20s %s", ui.Green("✓"), c.name, ui.Dim(detail))
		}
	}
	ui.Println("")
	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	ui.Success("Everything looks good")
	return nil
}

// reach succeeds when the URL answers at all; a registry answers 401 to
// anonymous requests, which still proves it is reachable.
func reach(url string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", ui.NewError("not reachable", "Check the internet connection, DNS and firewall of this machine.", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 500 {
		return "", fmt.Errorf("answered HTTP %d", resp.StatusCode)
	}
	return url, nil
}

func installState(dir, container string) (string, error) {
	if _, err := envfile.Load(dir + "/.env"); err != nil {
		return "not installed (" + dir + ")", nil
	}
	state := docker.ContainerState(container)
	switch state {
	case "healthy", "running":
		return state + " (" + dir + ")", nil
	case "starting":
		return "starting (" + dir + ")", nil
	}
	return "", fmt.Errorf("installed in %s but %s", dir, state)
}

func coreEnv() *envfile.File {
	env, err := envfile.Load(paths.CoreDir() + "/.env")
	if err != nil {
		return envfile.New("")
	}
	return env
}

func licenseAPI() string {
	if v := coreEnv().Get("LICENSE_API_URL"); v != "" {
		return v
	}
	return core.DefaultLicenseAPI
}

func registry() string {
	if v := coreEnv().Get("DECKPLANE_REGISTRY"); v != "" {
		return v
	}
	return core.DefaultRegistry
}

func traefikState() (string, error) {
	found := docker.FindTraefik("")
	if len(found) == 0 {
		return "not running (Deckplane can run its own)", nil
	}
	t := found[0]
	return fmt.Sprintf("%s · networks %v · resolver %q · entrypoint %q", t.Container, t.Networks, t.CertResolver, t.Entrypoint), nil
}
