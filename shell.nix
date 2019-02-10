let
  # Look here for information about how to generate `nixpkgs-version.json`.
  #  → https://nixos.wiki/wiki/FAQ/Pinning_Nixpkgs
  pinnedVersion = builtins.fromJSON (builtins.readFile ./.nixpkgs-version.json);
  pinnedPkgs = import (builtins.fetchGit {
    inherit (pinnedVersion) url rev;

    ref = "nixos-unstable";
  }) {};
in

# This allows overriding pkgs by passing `--arg pkgs ...`
{ pkgs ? pinnedPkgs }:

with pkgs;

mkShell {
  buildInputs = [
      nodejs-12_x
      yarn
      go_1_14
  ];
  # TODO: figure this out, maybe using buildGoModule is the way to go here instead ??
  # GOPATH = "/Users/blaiserivet/Documents/Blaise/dev/go";
  # TODO: explain nixify usage and envrc better somewhere
  shellHook = ''
      export GOPATH="$PWD/.go-cache"
  '';
}

