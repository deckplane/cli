# Deckplane CLI

**English** · [Türkçe](README-tr.md)

The `deckplane` command installs and manages [Deckplane](https://deckplane.com) on your own servers. It sets up **Deckplane Core** (the self-hosted control plane) and the **Deckplane agent** with Docker, and asks for anything it needs along the way, so no coding or Docker experience is required.

```bash
deckplane core install --license-key dpl_...
```

## What you need

- A Linux server (Ubuntu, Debian, etc.) or a computer running Docker Desktop (macOS, Windows)
- [Docker](https://docs.docker.com/get-docker/) with the Compose plugin. On Linux: `curl -fsSL https://get.docker.com | sudo sh`
- A license key from [Deckplane Cloud](https://cloud.deckplane.com) → **Licenses** (starts with `dpl_`)

Not sure the machine is ready? Run `deckplane doctor`; it checks everything and tells you how to fix what is missing.

## Install the CLI

**Linux and macOS**

```bash
curl -fsSL https://raw.githubusercontent.com/deckplane/deckplane-cli/main/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/deckplane/deckplane-cli/main/install.ps1 | iex
```

**With Go 1.26+**

```bash
go install github.com/deckplane/deckplane-cli@latest
```

Check it works with `deckplane version`.

The installers verify the download's checksum. To install a specific version, set `DECKPLANE_VERSION=v0.1.0` before running them; set `DECKPLANE_INSTALL_DIR` to choose the folder. Run the installer again at any time to update the CLI.

## Install Deckplane Core

```bash
sudo deckplane core install
```

The installer:

1. checks Docker,
2. asks for your license key (hidden while you type) and checks it with Deckplane,
3. asks how people will reach Core: a port on this machine (simplest) or your own domain through Traefik,
4. generates all passwords and secrets for you,
5. downloads and starts Core and its database, and waits until it is healthy,
6. offers to create your first admin account.

At the end it prints the address to open. Running `install` again is safe: your data and secrets are kept.

> On Linux, `sudo` is needed because Core is installed in `/var/lib/deckplane`. On macOS and Windows it goes to `~/.deckplane/core` and no `sudo` is needed.

### Without questions (automation)

```bash
export DECKPLANE_LICENSE_KEY=dpl_...
export DECKPLANE_ADMIN_PASSWORD='a-strong-password'
sudo -E deckplane core install --yes --port 3000 \
  --admin-email admin@example.com --admin-name "Admin"
```

Use `--domain deckplane.example.com --traefik-network traefik` instead of `--port` to serve Core on your domain through an existing Traefik.

## Connect a server (agent)

Run this on every Docker host you want to manage:

```bash
sudo deckplane agent install --server-url https://deckplane.example.com --token <token>
```

Print the token on the Core machine with `deckplane core token`. If Core runs on the same machine, `deckplane agent install` finds it on its own.

## Commands

| Command | What it does |
|---|---|
| `deckplane core install` | Install Deckplane Core |
| `deckplane core status` | Show whether Core is running, its address and version |
| `deckplane core logs [-f]` | Show Core's logs (`-f` keeps following) |
| `deckplane core start` / `stop` / `restart` | Start, stop, or restart Core |
| `deckplane core update [--version v1.2.3]` | Update to the latest or a given version |
| `deckplane core setup` | Create the first admin account |
| `deckplane core token` | Print the token agents connect with |
| `deckplane core config` | Show settings (secrets masked; `--reveal` shows them) |
| `deckplane core config set KEY=VALUE --restart` | Change a setting and apply it |
| `deckplane core license dpl_...` | Replace the license key |
| `deckplane core uninstall [--remove-data]` | Remove Core; data is kept unless `--remove-data` |
| `deckplane agent install` | Connect this Docker host to Core |
| `deckplane agent status` / `logs` / `update` / `uninstall` | Manage the agent |
| `deckplane doctor` | Check that this machine is ready |
| `deckplane version` | Print the CLI version |
| `deckplane completion bash\|zsh\|fish\|powershell` | Print a shell completion script |

Add `--help` to any command to see its options, and `-v` to see full Docker output. Every command accepts `--dir` to work with an install in another folder.

## Where things are

| | Linux | macOS / Windows |
|---|---|---|
| Core | `/var/lib/deckplane` | `~/.deckplane/core` |
| Agent | `/var/lib/deckplane-agent` | `~/.deckplane/agent` |

Each folder holds a `.env` (settings and secrets, readable only by its owner) and a generated `compose.yaml`. Do not edit `compose.yaml`; it is rewritten on install and update. Put your own Compose changes in `compose.override.yaml` next to it, and Docker Compose merges them in.

## Troubleshooting

- **"Docker is not installed" or "not running"**: the message tells you the exact command or app to start.
- **"Your license key was not accepted"**: check in Deckplane Cloud → Licenses that the key is active and not expired or revoked, and that you copied all of it.
- **"Port 3000 is already in use"**: install on another port, e.g. `--port 8080`.
- **Core does not become healthy**: run `deckplane core logs` to see why.
- Anything else: run `deckplane doctor`, then [open an issue](https://github.com/deckplane/deckplane-cli/issues) with its output.

## Development

```bash
go build -o deckplane .
go test ./...
./deckplane --help
```

To test against a locally built Core image, set `DECKPLANE_DEV=1`. This skips the registry sign-in and ignores pull failures, so you can use, for example, `--registry localhost --version dev` with an image tagged `localhost/deckplane/core:dev`.

### Releasing

Push a version tag and GitHub Actions publishes binaries for Linux, macOS and Windows (amd64 and arm64) with a `checksums.txt`:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Try a release locally with `goreleaser release --snapshot --clean`.

## Built with Kommando

The command tree is built with [Kommando](https://github.com/yigit433/kommando), a minimalist CLI framework for Go.

## License

[Apache License 2.0](LICENSE) © 2026 Meya Teknoloji
