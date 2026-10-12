#!/bin/sh
# Installs the deckplane CLI on Linux and macOS:
#
#   curl -fsSL https://raw.githubusercontent.com/deckplane/cli/main/install.sh | sh
#
# Optional environment variables:
#   DECKPLANE_VERSION       release to install, e.g. v0.1.0 (default: latest)
#   DECKPLANE_INSTALL_DIR   where to put the binary (default: /usr/local/bin)
#   DECKPLANE_RELEASES_URL  release download base (default: GitHub releases)
#
# Everything runs inside main(), so a partially downloaded script does nothing.

set -eu

REPO="deckplane/cli"

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
	BOLD="$(printf '\033[1m')"; GREEN="$(printf '\033[32m')"; RED="$(printf '\033[31m')"
	YELLOW="$(printf '\033[33m')"; CYAN="$(printf '\033[36m')"; RESET="$(printf '\033[0m')"
else
	BOLD=""; GREEN=""; RED=""; YELLOW=""; CYAN=""; RESET=""
fi

step() { printf '%s→%s %s\n' "$CYAN" "$RESET" "$1"; }
ok() { printf '%s✓%s %s\n' "$GREEN" "$RESET" "$1"; }
warn() { printf '%s!%s %s\n' "$YELLOW" "$RESET" "$1"; }
fail() {
	printf '\n%s✗ %s%s\n' "$RED" "$1" "$RESET" >&2
	[ -n "${2:-}" ] && printf '  %s\n' "$2" >&2
	exit 1
}

# ask QUESTION: yes/no question answered from the terminal even when this
# script is piped into sh. Returns 1 when nobody can answer.
ask() {
	# Probe in a subshell: a failed redirection on ":" would exit the whole script.
	( : </dev/tty >/dev/tty ) 2>/dev/null || return 1
	printf '%s?%s %s %s(Y/n)%s ' "$CYAN" "$RESET" "$1" "$BOLD" "$RESET" >/dev/tty
	read -r answer </dev/tty || return 1
	case "$answer" in "" | [Yy]*) return 0 ;; *) return 1 ;; esac
}

# as_root CMD...: run a command as root, using sudo when needed.
as_root() {
	if [ "$(id -u)" -eq 0 ]; then
		"$@"
	elif command -v sudo >/dev/null 2>&1; then
		sudo "$@"
	else
		return 1
	fi
}

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d ' ' -f 1
	else
		shasum -a 256 "$1" | cut -d ' ' -f 1
	fi
}

detect_platform() {
	case "$(uname -s)" in
		Linux) OS=linux ;;
		Darwin) OS=darwin ;;
		*) fail "Unsupported operating system: $(uname -s)" \
			"On Windows, run in PowerShell:  irm https://raw.githubusercontent.com/$REPO/main/install.ps1 | iex" ;;
	esac
	case "$(uname -m)" in
		x86_64 | amd64) ARCH=amd64 ;;
		aarch64 | arm64) ARCH=arm64 ;;
		*) fail "Unsupported processor: $(uname -m)" "Deckplane supports 64-bit Intel/AMD and ARM processors." ;;
	esac
}

resolve_version() {
	if [ -n "${DECKPLANE_VERSION:-}" ]; then
		VERSION="$DECKPLANE_VERSION"
		return
	fi
	# Follow the /latest redirect instead of the GitHub API, which is rate limited.
	url="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest")" ||
		fail "Could not reach GitHub to find the latest version" "Check this machine's internet connection."
	VERSION="${url##*/}"
	case "$VERSION" in
		v*) ;;
		*) fail "No release of the deckplane CLI was found" "See https://github.com/$REPO/releases" ;;
	esac
}

download() {
	base="${DECKPLANE_RELEASES_URL:-https://github.com/$REPO/releases}/download/$VERSION"
	archive="deckplane_${OS}_${ARCH}.tar.gz"
	step "Downloading deckplane $VERSION for $OS/$ARCH"
	curl -fsSL -o "$TMP/$archive" "$base/$archive" ||
		fail "Download failed: $base/$archive" "Check the version name and this machine's internet connection."
	curl -fsSL -o "$TMP/checksums.txt" "$base/checksums.txt" ||
		fail "Could not download checksums.txt for $VERSION"

	expected="$(grep " $archive\$" "$TMP/checksums.txt" | cut -d ' ' -f 1)"
	[ -n "$expected" ] || fail "$archive is not listed in checksums.txt"
	[ "$(sha256 "$TMP/$archive")" = "$expected" ] ||
		fail "The download is corrupted (checksum mismatch)" "Please run the installer again."
	ok "Download verified"

	tar -xzf "$TMP/$archive" -C "$TMP" deckplane
	chmod 755 "$TMP/deckplane"
}

install_binary() {
	dir="${DECKPLANE_INSTALL_DIR:-/usr/local/bin}"
	step "Installing to $dir"
	if { [ -d "$dir" ] || mkdir -p "$dir" 2>/dev/null; } && [ -w "$dir" ]; then
		mv "$TMP/deckplane" "$dir/deckplane"
	elif as_root mkdir -p "$dir" && as_root mv "$TMP/deckplane" "$dir/deckplane"; then
		:
	else
		dir="$HOME/.local/bin"
		warn "No administrator rights; installing to $dir instead"
		mkdir -p "$dir"
		mv "$TMP/deckplane" "$dir/deckplane"
	fi
	BIN="$dir/deckplane"
	ok "Installed $("$BIN" version)"

	case ":$PATH:" in
		*":$dir:"*) ;;
		*) warn "$dir is not in your PATH. Add it with:  export PATH=\"$dir:\$PATH\"" ;;
	esac
}

check_docker() {
	if command -v docker >/dev/null 2>&1; then
		ok "Docker is installed"
		return
	fi
	warn "Docker is not installed. Deckplane runs on Docker."
	if [ "$OS" = darwin ]; then
		printf '  Install Docker Desktop: https://docs.docker.com/desktop/setup/install/mac-install/\n'
		return
	fi
	if ask "Install Docker now with the official script (get.docker.com)?"; then
		step "Installing Docker"
		curl -fsSL https://get.docker.com | as_root sh ||
			fail "Docker installation failed" "See https://docs.docker.com/engine/install/"
		ok "Docker installed"
	else
		printf '  Install it later with:  curl -fsSL https://get.docker.com | sudo sh\n'
	fi
}

main() {
	command -v curl >/dev/null 2>&1 || fail "curl is required" "Install it, e.g.  sudo apt-get install -y curl"
	command -v tar >/dev/null 2>&1 || fail "tar is required"

	printf '\n%sDeckplane CLI installer%s\n\n' "$BOLD" "$RESET"
	detect_platform
	resolve_version
	TMP="$(mktemp -d)"
	trap 'rm -rf "$TMP"' EXIT INT TERM
	download
	install_binary
	check_docker

	# Core installs into /var/lib on Linux, which needs root.
	sudo_prefix=""
	if [ "$OS" = linux ] && [ "$(id -u)" -ne 0 ]; then
		sudo_prefix="sudo "
	fi
	# sudo does not search ~/.local/bin, so use the full path there.
	cmd="deckplane"
	case ":$PATH:" in *":$(dirname "$BIN"):"*) ;; *) cmd="$BIN" ;; esac
	[ -n "$sudo_prefix" ] && [ "$(dirname "$BIN")" != /usr/local/bin ] && cmd="$BIN"
	printf '\n%sAll set!%s Install Deckplane Core with:\n\n  %s%s%s core install%s\n\n' \
		"$GREEN" "$RESET" "$BOLD" "$sudo_prefix" "$cmd" "$RESET"
	printf 'Not sure this machine is ready? Run:  deckplane doctor\n\n'
}

main "$@"
