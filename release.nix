{ system ? builtins.currentSystem }:
# { system ? "x86_64-linux" }:
let
  pkgs = let
    sources = import ./nix/sources.nix;
  in import sources.nixpkgs {
    inherit system;
  };
  inherit (pkgs) lib;

  buildYarn = { isWorkspacePackage ? false, src, name }:
    let
      lockfileToNpmDeps = lockfile: (
        pkgs.runCommandNoCC "yarn.lock.nix" {} ''
          ${pkgs.yarn2nix}/bin/yarn2nix --lockfile=${lockfile} > $out
        ''
      );
      myNpmDepsDrv = lockfileToNpmDeps "${src}/yarn.lock";
      myNpmDeps = (pkgs.callPackage myNpmDepsDrv {});
      yarnFlags = [
        "--offline"
        "--frozen-lockfile"
        "--ignore-engines"
        # "--ignore-scripts"
        "--non-interactive"
      ];
    in pkgs.stdenv.mkDerivation ({
      inherit name;
      src = pkgs.nix-gitignore.gitignoreSource [] src;

      buildInputs = [
        pkgs.nodejs-12_x
        pkgs.yarn
      ];

      configurePhase = ''
        # Yarn writes cache directories etc to $HOME.
        export HOME=$NIX_BUILD_TOP/yarn_home

        # Make yarn install packages from our offline cache, not the registry
        yarn config --offline set yarn-offline-mirror ${myNpmDeps.offline_cache}

        # Fixup "resolved"-entries in yarn.lock to match our offline cache
        ${pkgs.fixup_yarn_lock}/bin/fixup_yarn_lock yarn.lock

        yarn install ${lib.escapeShellArgs yarnFlags}
      '';

      buildPhase = ''
        runHook preBuild

        yarn ${lib.escapeShellArgs yarnFlags} build

        runHook postBuild
      '';

      installPhase = ''
        cp -r dist/ $out
      '';
    } // lib.attrsets.optionalAttrs isWorkspacePackage {
      preBuild = ''
        cd ${name}
      '';
    });

  autocontract_backend = pkgs.buildGoModule {
    pname = "autocontract";
    version = "0.0.1";

    src = ./src/backend;

    # Use GOFLAGS in preBuild as multiple tags with the 'buildFlags' attribute did not work out.
    preBuild = ''
      export GOFLAGS="-tags=netgo"
    '';

    buildFlagsArray = [
      "-ldflags=-extldflags \"-static\""
    ];

    # Don't forget to update this when the underlying dependencies change.
    # Using nixpkg's lib.fakeSha256 is an easy way to see what the new hash is.
    modSha256 = "0im0wn3avrb0nb85dfw0cxbkqgrcl0m553368ns498942saih3gq";
  };

  pdf_gen_chromium = pkgs.stdenv.mkDerivation {
    name = "pdf-gen-chromium";

    src = pkgs.nix-gitignore.gitignoreSource [] ./src/contract-templates/fonts;

    installPhase = ''
      cp -r ./ $out/
    '';
  };

  caddy_proxy = pkgs.stdenv.mkDerivation {
    name = "caddy-proxy";
    src = ./src/caddy-proxy;
    installPhase = ''
      cp Caddyfile $out
    '';
  };
in
{
  inherit autocontract_backend;
  inherit pdf_gen_chromium;
  inherit caddy_proxy;
  autocontract_front_end = buildYarn { isWorkspacePackage = true; src = ./src/js; name = "web"; };
  autocontract_template = buildYarn { src = ./src/contract-templates; name = "autocontract_template"; };
  docteurqui_homepage = buildYarn { isWorkspacePackage = true; src = ./src/js; name = "docteurqui-homepage"; };
}
