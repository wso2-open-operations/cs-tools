#!/usr/bin/env sh
# Download and install the latest uc release for the current platform.
set -eu

REPO=${UC_REPO:-pavinduLakshan/utilscli}
BIN_DIR=${UC_BIN_DIR:-"$HOME/.local/bin"}

case "$(uname -s)" in
  Darwin) OS=darwin ;;
  Linux)  OS=linux ;;
  *)      printf 'uc: unsupported operating system: %s\n' "$(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  arm64|aarch64) ARCH=arm64 ;;
  x86_64|amd64)  ARCH=amd64 ;;
  *)             printf 'uc: unsupported architecture: %s\n' "$(uname -m)" >&2; exit 1 ;;
esac

ARCHIVE="uc_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/latest/download/${ARCHIVE}"
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT HUP INT TERM

printf 'Downloading %s...\n' "$URL"
curl -fsSL --retry 3 "$URL" -o "$TMP_DIR/$ARCHIVE"
tar -xzf "$TMP_DIR/$ARCHIVE" -C "$TMP_DIR"

if [ ! -f "$TMP_DIR/uc" ]; then
  printf 'uc: release archive did not contain the uc binary\n' >&2
  exit 1
fi

mkdir -p "$BIN_DIR"
mv "$TMP_DIR/uc" "$BIN_DIR/uc"
chmod +x "$BIN_DIR/uc"

case "${SHELL:-}" in
  */zsh) PROFILE="$HOME/.zprofile" ;;
  */bash) PROFILE="$HOME/.bashrc" ;;
  *)      PROFILE="$HOME/.profile" ;;
esac

PATH_LINE='export PATH="$HOME/.local/bin:$PATH" # added by uc installer'
if [ "$BIN_DIR" = "$HOME/.local/bin" ] && ! grep -Fqs '.local/bin' "$PROFILE" 2>/dev/null; then
  printf '\n%s\n' "$PATH_LINE" >> "$PROFILE"
  printf 'Installed uc to %s and added it to PATH in %s.\n' "$BIN_DIR/uc" "$PROFILE"
  printf 'Open a new terminal (or run: . %s) before using uc.\n' "$PROFILE"
else
  printf 'Installed uc to %s.\n' "$BIN_DIR/uc"
  case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *) printf 'Add %s to your PATH to invoke uc from anywhere.\n' "$BIN_DIR" ;;
  esac
fi
