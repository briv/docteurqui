let
  sources = import ./nix/sources.nix;
  pkgs = import sources.nixpkgs {};
in
pkgs.mkShell {
  buildInputs = [
    pkgs.nodejs-12_x
    pkgs.yarn
    pkgs.go_1_14
  ];

  # TODO: figure this out, maybe using buildGoModule is the way to go here instead ??
  # GOPATH = "/Users/blaiserivet/Documents/Blaise/dev/go";
  # TODO: explain nixify usage and envrc better somewhere
  shellHook = ''
    export GOPATH="$PWD/.go-cache"
  '';
}
