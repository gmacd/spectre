# AI Setup
If you're working with pi, you can build and run the dockerfile with:

Build:
```bash
  docker build -t pi-sandbox -f Dockerfile.pi .
```

Run:
```bash
  docker run --rm -it \
    -v ~/.ssh/id_ed25519:/root/.ssh/id_ed25519:ro \
    -v "$(pwd):/workspace" \
    -v "$HOME/.pi/agent:/root/.pi/agent" \
    pi-sandbox
```
