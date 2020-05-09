#!/usr/bin/env bash

set -euo pipefail

THIS_DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

TMP_DIR=$(mktemp -d)
echo "$TMP_DIR"
cleanup_on_exit() {
    rm -rf "$TMP_DIR"
}
trap cleanup_on_exit EXIT INT TERM

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
    make_and_save_docker_image "autocontract/pdf-gen:latest" autocontract-pdf-gen_latest.tar.gz
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
    make_and_save_docker_image "autocontract/app:latest" autocontract-app_latest.tar.gz
}

make_autocontract_caddy_image () {
    setup_for_docker_image "caddy-proxy"

    nix build \
        --out-link docteurqui_homepage \
        --file "$THIS_DIR/release.nix" \
        docteurqui_homepage

    nix build \
        --out-link caddy_proxy \
        --file "$THIS_DIR/release.nix" \
        caddy_proxy

    cp caddy_proxy/Caddyfile Caddyfile
    mkdir -p www-root/static
    cp caddy_proxy/robots.txt www-root/robots.txt
    cp -R docteurqui_homepage/ www-root/static
    mv www-root/static/index.html www-root/index.html

    cp "$THIS_DIR/docker/caddy/Dockerfile" Dockerfile
    make_and_save_docker_image "autocontract/caddy:latest" caddy_latest.tar.gz
}

make_autocontract_caddy_image
make_autocontract_app_docker_image
# make_autocontract_pdf_gen_chromium_image

cd "$TMP_DIR"
# TODO: maybe something nicer than "|| true" as rsync still clutters our output when no tar files are present.
rsync --progress *.tar.gz briv@vultr:/var/lib/docteurqui/docker-images/ || true
scp "$THIS_DIR/deploying/censor.secret" vultr://var/lib/docteurqui/
scp "$THIS_DIR/deploying/chrome_seccomp.json" vultr://var/lib/docteurqui/
scp "$THIS_DIR/deploying/docker-services-config.nix" vultr://home/briv/
scp "$THIS_DIR/deploying/configuration.nix" vultr://etc/nixos/configuration.nix
