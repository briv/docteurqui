{ config, pkgs, ... }:
let
  dockerNetworks = {
    internalPdf = "pdf-net";
  };
  dockerImagesFolder = /var/lib/docteurqui/docker-images;
  dockerVolumes = {
    caddyConfig = "caddy-config";
    caddyData = "caddy-data";
  };
in
{
  virtualisation.docker.enable = true;

  systemd.services = {
    init-docker-related-stuff = {
      description =
        "Creates directories, docker networks and docker volumes for the setup to work.";
      after = [ "network.target" ];
      wantedBy = [ "multi-user.target" ];

      serviceConfig.Type = "oneshot";
      script =
        let
          dockercli = "${config.virtualisation.docker.package}/bin/docker";
        in ''
          mkdir -p "${builtins.toString dockerImagesFolder}"

          ${dockercli} load --input "${builtins.toString (dockerImagesFolder + /caddy_latest.tar.gz)}"

          # Put a true at the end to prevent getting non-zero return code, which will
          # crash the whole service.
          check=$(${dockercli} network ls | grep "${dockerNetworks.internalPdf}" || true)
          if [ -z "$check" ]; then
            ${dockercli} network create --internal ${dockerNetworks.internalPdf}
          else
            echo "'${dockerNetworks.internalPdf}' network already exists in docker"
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
        '';
    };
  };

  docker-containers."caddy" = {
    image = "autocontract/caddy:latest";
    # not yet supported on our NixOS version
    # imageFile = builtins.path {
    #   path = (dockerImagesFolder + /caddy_latest.tar.gz);
    # };
    environment = {
      # IMPORTANT: this
      "AUTOCONTRACT_HOSTNAME" = "docker-filerun-mariadb.services";
    };
    ports = [
      "80:80"
      "443:443"
    ];
    volumes = [
      "${dockerVolumes.caddyData}:/data"
      "${dockerVolumes.caddyConfig}:/config"
    ];
    # extraDockerOptions = [ "--network=${dockerNetworks.internalPdf}" ];
  };
}

#   docker-containers."filerun" = {
#     image = "afian/filerun";
#     environment = {
#       "FR_DB_HOST" = "docker-filerun-mariadb.services"; # !! IMPORTANT
#       "FR_DB_PORT" = "3306";
#       "FR_DB_NAME" = "filerundb";
#       "FR_DB_USER" = "filerun";
#       "FR_DB_PASS" = "randompasswd";
#       "APACHE_RUN_USER" = "delegator";
#       "APACHE_RUN_USER_ID" = "600";
#       "APACHE_RUN_GROUP" = "delegator";
#       "APACHE_RUN_GROUP_ID" = "600";
#     };
#     ports = [ "6000:80" ];
#     volumes = [
#       "/home/delegator/filerun/web:/var/www/html"
#       "/home/delegator/filerun/user-files:/user-files"
#     ];
#     extraDockerOptions = [ "--network=filerun-br" ];
#   };

#   docker-containers."filerun-mariadb" = {
#     image = "mariadb:10.1";
#     environment = {
#       "MYSQL_ROOT_PASSWORD" = "randompasswd";
#       "MYSQL_USER" = "filerun";
#       "MYSQL_PASSWORD" = "randompasswd";
#       "MYSQL_DATABASE" = "filerundb";
#     };
#     volumes = [ "/home/delegator/filerun/db:/var/lib/mysql" ];
#     extraDockerOptions = [ "--network=filerun-br" ];
#   };
#   };
# }
