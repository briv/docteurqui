#!/usr/bin/env bash

set -euo pipefail

THIS_DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

TMP_DIR=$(mktemp -d)
echo "$TMP_DIR"
cleanup_on_exit() {
    true
    # echo "$TMP_DIR"
    # file "$TMP_DIR/autocontract-app-docker/autocontract"
    # rm -r "$TMP_DIR"
}
trap cleanup_on_exit EXIT INT TERM ERR

setup_for_docker_image () {
    cd "$TMP_DIR"
    local DOCKER_BUILD_DIR="$1-docker"
    mkdir -p "$DOCKER_BUILD_DIR" && cd "$DOCKER_BUILD_DIR"
}

make_and_save_docker_image () {
    local IMAGE_TAG="$1"
    local TAR_FILE="$2"

    docker build --tag "$IMAGE_TAG" ./
    docker save "$IMAGE_TAG" | gzip > "$TAR_FILE"
    mv "$TAR_FILE" "../$TAR_FILE"
}


make_autocontract_pdf_gen_chromium_image () {
    setup_for_docker_image "pdf-gen-chromium"

    nix build \
        --argstr "system" "x86_64-linux" \
        --out-link pdf_gen_chromium \
        --file "$THIS_DIR/release.nix" \
        pdf_gen_chromium

    mkdir fonts
    cp -r pdf_gen_chromium/ fonts
    cp "$THIS_DIR/docker/pdf-gen-chromium/Dockerfile" Dockerfile
    docker build --tag "autocontract/pdf-gen" ./
}

make_autocontract_app_docker_image () {
    setup_for_docker_image "autocontract-app"

    # autocontract golang binary
    nix build \
        --argstr "system" "x86_64-linux" \
        --out-link autocontract_backend \
        --file "$THIS_DIR/release.nix" \
        autocontract_backend

    # nix-build -vvvv \
    #     --argstr "system" "x86_64-linux" \
    #     --out-link autocontract_backend \
    #     "$THIS_DIR/release.nix" \
    #     -A autocontract_backend

    cp autocontract_backend/bin/autocontract autocontract

    # front-end files
    nix build \
        --out-link autocontract_front_end \
        --file "$THIS_DIR/release.nix" \
        autocontract_front_end

    mkdir www
    cp -R autocontract_front_end/ www

    # contract template file
    nix build \
        --out-link autocontract_template \
        --file "$THIS_DIR/release.nix" \
        autocontract_template

    cp autocontract_template/index.html contract-template.html

    # build docker image
    cp "$THIS_DIR/docker/autocontract-app/Dockerfile" Dockerfile
    docker build --tag "autocontract/app" ./
}

make_autocontract_caddy_image () {
    setup_for_docker_image "caddy-proxy"

    nix build \
        --out-link docteurqui_homepage \
        --file "$THIS_DIR/release.nix" \
        docteurqui_homepage

    nix build \
        --out-link ourcaddyfile \
        --file "$THIS_DIR/release.nix" \
        caddy_proxy

    cp ourcaddyfile Caddyfile
    mkdir -p www-root/static
    cp -r docteurqui_homepage/ www-root/static
    mv www-root/static/index.html www-root/index.html

    cp "$THIS_DIR/docker/caddy/Dockerfile" Dockerfile
    make_and_save_docker_image "autocontract/caddy:latest" caddy_latest.tar.gz
}

# make_autocontract_app_docker_image
# make_autocontract_pdf_gen_chromium_image
make_autocontract_caddy_image

cd "$TMP_DIR"
# something like this rsync command updates all our docker images:
# rsync *.tar.gz briv@vultr:/var/lib/docteurqui/docker-images/
