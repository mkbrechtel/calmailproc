#!/bin/sh -e
    
podman build -t calmailproc-devenv -f devenv.Containerfile --target devenv .

# Create a named volume for the home directory
if ! podman volume ls | grep -q "devenv-root-home"; then
    podman volume create devenv-root-home
fi

exec podman run -it --replace \
    --name calmailproc-devenv \
    --hostname calmailproc-devenv \
    -v claude-root-home:/root \
    -v ./:/mnt/calmailproc/ \
    --workdir /mnt/calmailproc \
    calmailproc-devenv \
    fish
