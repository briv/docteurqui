{ config, pkgs, ... }:
{
  # Needed for IPv6
  config.boot.kernel.sysctl."net.ipv6.conf.all.forwarding" = "1";

  config.networking.nat =
    let
      iface = "wg0";
      oface = "ens3";
    in {
      enable = true;
      externalInterface = oface;
      internalInterfaces = [ iface ];

      # for Wireguard with IPv6
      extraCommands = ''
        # Manually added for IPv6 with Wireguard.
        # Here we mark all packets coming from the wireguard interface.
        ip6tables -w -t nat -A nixos-nat-pre \
            -i '${iface}' -j MARK --set-mark 1

        # See wireguard.nix for "-j MASQUERADE". That value could change based
        # on wireguard configuration.
        # Here all previously marked packets that are exiting through the
        # "oface" interface are masqueraded (i.e. source address will be this
        # host).
        ip6tables -w -t nat -A nixos-nat-post \
            -m mark --mark 1 -o '${oface}' -j MASQUERADE
      '';
    };

  config.networking.wireguard.interfaces.wg0 = {
    ips = [ "10.9.8.1/24" "fda8:3f56:9f5d:86e9::1/64" ];
    listenPort = 60991;
    privateKeyFile = "/home/briv/wg-priv.key";
    peers = (import ./wireguard-peers.nix { inherit (pkgs) lib; });
  };
}
