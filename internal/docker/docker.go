// Package docker wraps the docker CLI. We shell out instead of using the
// Docker SDK so the user's own Docker context, credentials and Compose plugin
// are used exactly as they would be from the terminal.
package docker

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/deckplane/deckplane-cli/internal/ui"
)

// Check verifies that Docker is installed, the daemon is running and the
// current user may talk to it. The returned error explains how to fix it.
func Check() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return ui.NewError("Docker is not installed", installHint(), nil)
	}
	out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").CombinedOutput()
	if err == nil {
		return nil
	}
	msg := string(out)
	switch {
	case strings.Contains(msg, "permission denied"):
		return ui.NewError("Your user is not allowed to use Docker",
			"Run the command with sudo, or add yourself to the docker group:\n  sudo usermod -aG docker $USER   (then log out and back in)", nil)
	default:
		hint := "Start Docker and try again."
		if runtime.GOOS == "linux" {
			hint = "Start it with:  sudo systemctl start docker"
		} else {
			hint = "Open Docker Desktop and wait until it says \"Docker is running\", then try again."
		}
		return ui.NewError("Docker is installed but not running", hint, nil)
	}
}

// CheckCompose verifies that the Docker Compose v2 plugin is available.
func CheckCompose() error {
	if err := exec.Command("docker", "compose", "version").Run(); err != nil {
		hint := "Install the Docker Compose plugin: https://docs.docker.com/compose/install/"
		if runtime.GOOS == "linux" {
			hint = "Install it with:  sudo apt-get install docker-compose-plugin\n(or see https://docs.docker.com/compose/install/linux/)"
		}
		return ui.NewError("Docker Compose v2 is not available", hint, nil)
	}
	return nil
}

func installHint() string {
	switch runtime.GOOS {
	case "linux":
		return "Install it with the official script:\n  curl -fsSL https://get.docker.com | sudo sh"
	case "darwin":
		return "Install Docker Desktop for Mac: https://docs.docker.com/desktop/setup/install/mac-install/"
	case "windows":
		return "Install Docker Desktop for Windows: https://docs.docker.com/desktop/setup/install/windows-install/"
	}
	return "See https://docs.docker.com/get-docker/"
}

// Version returns the Docker server version, or "" if unknown.
func Version() string {
	out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Project runs docker compose commands for one install directory.
type Project struct {
	Dir     string
	Verbose bool
}

func (p Project) command(args ...string) *exec.Cmd {
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Dir = p.Dir
	return cmd
}

// Run executes a compose command. Output is shown only in verbose mode or
// when the command fails.
func (p Project) Run(args ...string) error {
	cmd := p.command(args...)
	if p.Verbose {
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		return wrap(args, cmd.Run(), "")
	}
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	return wrap(args, cmd.Run(), buf.String())
}

// Stream executes a compose command and always shows its output (used for
// image pulls so long downloads show progress).
func (p Project) Stream(args ...string) error {
	cmd := p.command(args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return wrap(args, cmd.Run(), "")
}

// Attach runs a compose command connected to the user's terminal (logs -f).
func (p Project) Attach(args ...string) error {
	cmd := p.command(args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// Exec runs a command inside a service container, feeding stdin and
// returning stdout.
func (p Project) Exec(service string, stdin io.Reader, command ...string) ([]byte, error) {
	cmd := p.command(append([]string{"exec", "-T", service}, command...)...)
	cmd.Stdin = stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func wrap(args []string, err error, output string) error {
	if err == nil {
		return nil
	}
	msg := fmt.Sprintf("docker compose %s failed", args[0])
	if tail := lastLines(output, 15); tail != "" {
		return fmt.Errorf("%s:\n%s", msg, tail)
	}
	return fmt.Errorf("%s: %w", msg, err)
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// ErrUnauthorized means the registry rejected the credentials.
var ErrUnauthorized = errors.New("registry rejected the credentials")

// Login signs in to a registry with the password passed on stdin, so it never
// appears in the process list.
func Login(registry, username, password string) error {
	cmd := exec.Command("docker", "login", registry, "--username", username, "--password-stdin")
	cmd.Stdin = strings.NewReader(password)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	msg := strings.ToLower(string(out))
	if strings.Contains(msg, "unauthorized") || strings.Contains(msg, "denied") || strings.Contains(msg, "401") {
		return ErrUnauthorized
	}
	return fmt.Errorf("docker login %s failed: %s", registry, strings.TrimSpace(string(out)))
}

// Logout removes stored credentials for registry. Errors are ignored.
func Logout(registry string) { _ = exec.Command("docker", "logout", registry).Run() }

// ContainerState returns the health status when the container has a health
// check ("starting", "healthy", "unhealthy"), otherwise its state
// ("running", "exited", ...). It returns "missing" when there is no such
// container.
func ContainerState(name string) string {
	out, err := exec.Command("docker", "inspect", "--format",
		"{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}", name).Output()
	if err != nil {
		return "missing"
	}
	return strings.TrimSpace(string(out))
}

// ContainerImage returns the image reference a container was created from.
func ContainerImage(name string) string {
	out, err := exec.Command("docker", "inspect", "--format", "{{.Config.Image}}", name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Logs returns the last n log lines of a container.
func Logs(name string, n int) string {
	out, _ := exec.Command("docker", "logs", "--tail", fmt.Sprint(n), name).CombinedOutput()
	return strings.TrimSpace(string(out))
}

// BridgeNetworks lists user-visible bridge networks.
func BridgeNetworks() []string {
	out, err := exec.Command("docker", "network", "ls", "--filter", "driver=bridge", "--format", "{{.Name}}").Output()
	if err != nil {
		return nil
	}
	var nets []string
	for _, n := range strings.Fields(string(out)) {
		if n != "bridge" {
			nets = append(nets, n)
		}
	}
	return nets
}

// NetworkExists reports whether a Docker network with this name exists.
func NetworkExists(name string) bool {
	return exec.Command("docker", "network", "inspect", name).Run() == nil
}

// CreateNetwork creates a bridge network.
func CreateNetwork(name string) error {
	out, err := exec.Command("docker", "network", "create", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not create network %q: %s", name, strings.TrimSpace(string(out)))
	}
	return nil
}
