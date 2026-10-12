// Package buildinfo holds values stamped into the binary at release time:
//
//	go build -ldflags "-X github.com/deckplane/cli/internal/buildinfo.Version=v1.2.3"
package buildinfo

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)
