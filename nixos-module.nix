{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.services.neru;
  configFile = pkgs.writeText "config.toml" cfg.config;
in
{
  options = {
    services.neru = {
      enable = lib.mkEnableOption "Neru keyboard navigation";
      package = lib.mkPackageOption pkgs "neru" { };
      config = lib.mkOption {
        type = lib.types.lines;
        default = builtins.readFile ./configs/default-config.toml;
        description = "Config to use for {file} `neru/config.toml`.";
      };
    };
  };
  config = (
    lib.mkIf (cfg.enable) {
      environment.systemPackages = [ cfg.package ];

      systemd.user.services.neru = {
        description = "Neru keyboard navigation daemon";
        after = [ "graphical-session.target" ];
        partOf = [ "graphical-session.target" ];
        wantedBy = [ "graphical-session.target" ];
        serviceConfig = {
          ExecStart =
            "${cfg.package}/bin/neru launch"
            + (lib.optionalString (cfg.config != "") " --config ${configFile}");
          Restart = "on-failure";
          RestartSec = 5;
          Nice = -10;
        };
      };
    }
  );
}
