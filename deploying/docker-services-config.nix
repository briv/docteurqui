{ config, pkgs, ... }:
let
  paths = {
    dockerImagesFolder = /var/lib/docteurqui/docker-images;
    secretKeyFile = /var/lib/docteurqui/censor.secret;
    chromeSeccomp = /var/lib/docteurqui/chrome_seccomp.json;
  };
  dockerNetworks = {
    principal = "principal-net";
    internalPdf = "pdf-net";
  };
  dockerVolumes = {
    caddyConfig = "caddy-config";
    caddyData = "caddy-data";
    doctorData = "doctor-data";
  };
in
{
  virtualisation.docker = {
    enable = true;
    logDriver = "journald";
  };

  systemd.services = {
    init-docker-related-stuff = {
      description =
        "Creates directories, docker networks and docker volumes for the setup to work.";
      wantedBy = [ "multi-user.target" ];

      after = [ "docker.service" "docker.socket" ];
      requires = [ "docker.service" "docker.socket" ];

      before = [];

      serviceConfig.Type = "oneshot";
      script =
        let
          dockercli = "${config.virtualisation.docker.package}/bin/docker";
        in ''
          ${dockercli} load --input "${builtins.toString (paths.dockerImagesFolder + /caddy_latest.tar.gz)}"
          ${dockercli} load --input "${builtins.toString (paths.dockerImagesFolder + /autocontract-app_latest.tar.gz)}"
          ${dockercli} load --input "${builtins.toString (paths.dockerImagesFolder + /autocontract-pdf-gen_latest.tar.gz)}"

          # Put a true at the end to prevent getting non-zero return code, which will
          # crash the whole service.
          check=$(${dockercli} network ls | grep "${dockerNetworks.internalPdf}" || true)
          if [ -z "$check" ]; then
            ${dockercli} network create --internal ${dockerNetworks.internalPdf}
          else
            echo "'${dockerNetworks.internalPdf}' network already exists in docker"
          fi

          check=$(${dockercli} network ls | grep "${dockerNetworks.principal}" || true)
          if [ -z "$check" ]; then
            ${dockercli} network create ${dockerNetworks.principal}
          else
            echo "'${dockerNetworks.principal}' network already exists in docker"
          fi

          check=$(${dockercli} volume ls | grep "${dockerVolumes.caddyConfig}" || true)
          if [ -z "$check" ]; then
            ${dockercli} volume create ${dockerVolumes.caddyConfig}
          else
            echo "'${dockerVolumes.caddyConfig}' volume already exists in docker"
          fi

          check=$(${dockercli} volume ls | grep "${dockerVolumes.caddyData}" || true)
          if [ -z "$check" ]; then
            ${dockercli} volume create ${dockerVolumes.caddyData}
          else
            echo "'${dockerVolumes.caddyData}' volume already exists in docker"
          fi

          check=$(${dockercli} volume ls | grep "${dockerVolumes.doctorData}" || true)
          if [ -z "$check" ]; then
            ${dockercli} volume create ${dockerVolumes.doctorData}
          else
            echo "'${dockerVolumes.doctorData}' volume already exists in docker"
          fi
        '';
    };
  };

  docker-containers."caddy" = {
    image = "autocontract/caddy:latest";
    # not yet supported on our NixOS version
    # imageFile = builtins.path {
    #   path = (paths.dockerImagesFolder + /caddy_latest.tar.gz);
    # };
    environment = {
      # TODO: this is not great
      "AUTOCONTRACT_HOSTNAME" = "docker-autocontract-app.service";
    };
    ports = [
      "80:80"
      "443:443"
    ];
    volumes = [
      "${dockerVolumes.caddyData}:/data"
      "${dockerVolumes.caddyConfig}:/config"
    ];
    extraDockerOptions = [
      "--network=${dockerNetworks.principal}"
    ];
  };

  # TODO: Hacky attempt (uses sleep) to connect a container to a user network.
  # The issue is our systemd services wrapping docker are of Type=simple so
  # they are considered started and ready before even systemd calls.
  # Checkout out sdnotify-proxy or systemd-docker for some more readying on the subject.
  systemd.services."docker-autocontract-app".postStart =
    let
      dockercli = "${config.virtualisation.docker.package}/bin/docker";
    in "sleep 5 && ${dockercli} network connect ${dockerNetworks.internalPdf} docker-autocontract-app.service";

  docker-containers."autocontract-app" = {
    image = "autocontract/app:latest";
    environment = {
      "PDF_GEN_URL" = "http://docker-autocontract-pdf-gen.service:9222";
      "PDF_INTERNAL_WEB_HOSTNAME" = "docker-autocontract-app.service";
      "SECRET_CENSOR_KEY" = (builtins.readFile paths.secretKeyFile);
    };
    extraDockerOptions = [
      "--network=${dockerNetworks.principal}"
      "--mount=source=${dockerVolumes.doctorData},target=/docker-vols/doctor-data,readonly"
    ];
  };

  docker-containers."autocontract-pdf-gen" = {
    image = "autocontract/pdf-gen:latest";
    extraDockerOptions = [
      "--network=${dockerNetworks.internalPdf}"
      "--security-opt=no-new-privileges"
      "--security-opt=seccomp=${paths.chromeSeccomp}"
    ];
  };
}
