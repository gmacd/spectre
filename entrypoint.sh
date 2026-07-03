#!/usr/bin/env bash
set -euo pipefail

SSH_KEY="${SSH_KEY_PATH:-/root/.ssh/id_pub}"

# Start ssh-agent and expose its environment to child processes
if command -v ssh-agent &>/dev/null; then
  eval "$(ssh-agent -s)"
  if [ -f "$SSH_KEY" ]; then
    ssh-add "$SSH_KEY" 2>/dev/null || true
  fi
fi

exec "$@"
