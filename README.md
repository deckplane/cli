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
curl -fsSL https://raw.githubusercontent.com/deckplane/cli/main/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/deckplane/cli/main/install.ps1 | iex
```

**With Go 1.26+**

```bash
go install github.com/deckplane/cli@latest
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
3. asks how people will reach Core: your own domain over HTTPS, or this server's IP address (see below),
4. generates all passwords and secrets for you,
5. downloads and starts Core and its database, and waits until it is healthy,
6. offers to create your first admin account.

At the end it prints the address to open. Running `install` again is safe: your data and secrets are kept.

### Domain or IP address

Core never opens a port of its own: it always sits behind [Traefik](https://traefik.io). If a Traefik already runs on the server in Docker, the installer finds it, reads its network, certificate resolver and entrypoints, and asks you to confirm. If there is none, Deckplane runs its own.

| Choice | What you get | What you need |
|---|---|---|
| **Your domain** (recommended) | `https://deckplane.example.com` with a certificate | The domain's DNS pointing to this server. With your own Traefik, nothing else; with Deckplane's, ports 80 and 443 free and an email address for Let's Encrypt |
| **IP address, no domain** | `http://203.0.113.10` (plain HTTP on port 80) | Port 80 free, or your own Traefik's HTTP entrypoint. Good for trying Deckplane or private networks |

With your own Traefik and no domain, Core answers every request on port 80 that no other app claims. This does not work if your Traefik redirects all HTTP to HTTPS; use a domain then.

When Deckplane runs its own Traefik, other apps on the server can use it too: attach them to the `deckplane-proxy` Docker network and add Traefik labels (entrypoint `websecure` and certificate resolver `le` with a domain, entrypoint `web` without).

> On Linux, `sudo` is needed because Core is installed in `/var/lib/deckplane`. On macOS and Windows it goes to `~/.deckplane/core` and no `sudo` is needed.

### Without questions (automation)

```bash
export DECKPLANE_LICENSE_KEY=dpl_...
export DECKPLANE_ADMIN_PASSWORD='a-strong-password'
sudo -E deckplane core install --yes \
  --domain deckplane.example.com --acme-email you@example.com \
  --admin-email admin@example.com --admin-name "Admin"
```

A running Traefik is detected and used, and then `--acme-email` is not needed; otherwise Deckplane runs its own. When detection cannot read your Traefik's settings, pass them with `--traefik-network`, `--cert-resolver` and `--traefik-entrypoint`. Leave out `--domain` and `--acme-email` to serve Core on the server's IP address over HTTP.

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
- **"Port 80 (or 443) is already in use"**: Deckplane wants to run its own Traefik, but another program holds the port; the message names it when it can. If it is a Traefik running in Docker, use it with `--traefik-network <its network>`. Otherwise stop that program (for example `systemctl stop nginx`) and run the installer again.
- **The IP address does not open** (no domain, your own Traefik): your Traefik probably redirects all HTTP to HTTPS. Use a domain instead.
- **The domain does not open or shows a certificate warning**: its DNS A record must point to this server, and ports 80 and 443 must be reachable from the internet so Let's Encrypt can issue the certificate.
- **Core does not become healthy**: run `deckplane core logs` to see why.
- Anything else: run `deckplane doctor`, then [open an issue](https://github.com/deckplane/cli/issues) with its output.

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

## Contributing

Bug reports and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md); report security problems privately as described in [SECURITY.md](SECURITY.md).

## License

[Apache License 2.0](LICENSE) © 2026 Meya Teknoloji
