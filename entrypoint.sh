#!/usr/bin/env bash
set -euo pipefail

mkdir -p ~/.ssh
chmod 700 ~/.ssh

# Populate known_hosts so SSH host key verification doesn't fail
ssh-keyscan github.com >> ~/.ssh/known_hosts 2>/dev/null || true
SSH_KEY="${SSH_KEY_PATH:-/root/.ssh/id_github}"

# Start ssh-agent and expose its environment to child processes
if command -v ssh-agent &>/dev/null; then
  eval "$(ssh-agent -s)"
  if [ -f "$SSH_KEY" ]; then
    ssh-add "$SSH_KEY" 2>/dev/null || true
  fi
fi

exec "$@"
