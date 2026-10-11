// Package stack holds helpers shared by the Core and agent installers.
package stack

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/deckplane/deckplane-cli/internal/docker"
	"github.com/deckplane/deckplane-cli/internal/ui"
)

// RandomHex returns n random bytes encoded as 2n hex characters.
func RandomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

// WriteFile writes a generated file with owner-only permissions.
func WriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

// WaitHealthy polls a container until its health check passes. On failure
// it includes the container's last log lines in the error.
func WaitHealthy(container string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		switch state := docker.ContainerState(container); state {
		case "healthy":
			return nil
		case "unhealthy", "exited", "dead":
			return ui.NewError(fmt.Sprintf("%s failed to start (%s)", container, state),
				"Last log lines:\n"+indent(docker.Logs(container, 20)), nil)
		}
		time.Sleep(2 * time.Second)
	}
	return ui.NewError(fmt.Sprintf("%s did not become healthy within %s", container, timeout),
		"It may still be starting. Check its logs with:  deckplane core logs", nil)
}

func indent(s string) string {
	if s == "" {
		return "  (no logs)"
	}
	return "  " + strings.ReplaceAll(s, "\n", "\n  ")
}

// PortFree reports whether a TCP port can be bound on all interfaces. A
// permission error (low ports without root) says nothing about the port being
// used, so it counts as free.
func PortFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return errors.Is(err, os.ErrPermission)
	}
	l.Close()
	return true
}

// LocalIP returns this machine's primary outbound IP address, used to show a
// URL that works from other computers. No packets are sent.
func LocalIP() string {
	conn, err := net.Dial("udp", "1.1.1.1:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

// IsSecretKey reports whether an env key holds a secret that must not be
// printed in full.
func IsSecretKey(key string) bool {
	for _, s := range []string{"SECRET", "PASSWORD", "TOKEN", "KEY", "DATABASE_URL"} {
		if strings.Contains(key, s) {
			return true
		}
	}
	return false
}

// Mask hides all but the first few characters of a secret.
func Mask(v string) string {
	if v == "" {
		return ""
	}
	if len(v) <= 8 {
		return "••••"
	}
	return v[:4] + "••••"
}
