docker run --rm -it \
    -v ~/.ssh/id_ed25519_github_nopw:/root/.ssh/id_pub:ro \
    -v "$(pwd):/workspace" \
    -v "$HOME/.pi/agent:/root/.pi/agent" \
    pi-sandbox
