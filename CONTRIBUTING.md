# Contributing to the Deckplane CLI

Thanks for helping improve the Deckplane CLI! Bug reports, ideas and pull requests are all welcome. Türkçe yazabilirsiniz; issues and pull requests in Turkish are welcome too.

## Reporting a bug

[Open an issue](https://github.com/deckplane/cli/issues/new/choose) and include:

- the output of `deckplane version` and `deckplane doctor`,
- the command you ran and its full output (run it again with `-v` for Docker's output),
- your operating system.

Remove license keys, tokens and passwords before posting. `deckplane core config` masks them for you.

Security problems should not go in public issues; see [SECURITY.md](SECURITY.md).

## Development setup

You need Go (the version in `go.mod`) and Docker with the Compose plugin.

```bash
go build -o deckplane .
./deckplane --help
go test ./...
```

To try `deckplane core install` without the private registry, build a Core image locally and set `DECKPLANE_DEV=1` (see the README's Development section). Use `--dir` with a temporary folder and a free `--port` so you do not touch a real installation.

## Project layout

| Path | Contents |
|---|---|
| `cmd/` | Command tree and flags, built with [Kommando](https://github.com/yigit433/kommando) |
| `internal/core/` | Installing and managing Deckplane Core |
| `internal/agent/` | Installing and managing the Deckplane agent |
| `internal/doctor/` | `deckplane doctor` checks |
| `internal/docker/` | Wrappers around the `docker` and `docker compose` commands |
| `internal/envfile/` | Order-preserving `.env` reader and writer |
| `internal/ui/` | Terminal output, errors with hints, and prompts |
| `install.sh`, `install.ps1` | One-line installers that download release binaries |

## Guidelines

- **Write for people who do not code.** Messages should say what happened and what to do next. Return `ui.NewError(msg, hint, err)` with a hint the user can act on.
- **Never print secrets.** Mask them with `stack.Mask` and never pass passwords as command-line arguments (use stdin).
- **Keep install idempotent.** Running `install` again must keep existing data and secrets.
- **Don't edit generated files by hand in templates.** `compose.yaml` is regenerated on install and update; user changes belong in `compose.override.yaml`.
- Run `gofmt`, `go vet ./...` and `go test ./...` before opening a pull request. Changes to `install.sh` must pass `shellcheck install.sh`.
- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/): `feat(core): …`, `fix(agent): …`, `docs: …`.

## Pull requests

1. Fork the repository and create a branch from `main`.
2. Make your change, with tests where it makes sense.
3. Update `README.md` **and** `README-tr.md` if users will notice the change.
4. Open the pull request and describe what changed and how you tested it.

By contributing, you agree that your contributions are licensed under the [Apache License 2.0](LICENSE).
