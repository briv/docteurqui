#!/usr/bin/env bash

set -euo pipefail

THIS_DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

TMP_DIR=$(mktemp -d)
cleanup_on_exit() {
    true
    echo "$TMP_DIR"
    file "$TMP_DIR/autocontract-app-docker/autocontract"
    # rm -r "$TMP_DIR"
}
trap cleanup_on_exit EXIT INT TERM ERR

setup_for_docker_image () {
    cd "$TMP_DIR"
    local DOCKER_BUILD_DIR="$1-docker"
    mkdir -p "$DOCKER_BUILD_DIR" && cd "$DOCKER_BUILD_DIR"
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

make_autocontract_app_docker_image
make_autocontract_pdf_gen_chromium_image
