{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.services.neru;
  configFile =
    if cfg.configFile != null then cfg.configFile else pkgs.writeText "config.toml" cfg.config;
in
{
  options = {
    services.neru = {
      enable = lib.mkEnableOption "Neru keyboard navigation";
      package = lib.mkPackageOption pkgs "neru" { };
      config = lib.mkOption {
        type = lib.types.lines;
        default = builtins.readFile ./configs/default-config.toml;
        description = ''
          Config to use for {file} `neru/config.toml`.

          NOTE: The default config ships with macOS-style hotkeys (Cmd+Shift+…).
          On Linux you almost certainly want to override the [hotkeys] section
          with Ctrl-based or Primary-based shortcuts, e.g.:

            services.neru.config = '''
              [hotkeys]
              "Ctrl+Shift+Space" = "hints"
              "Ctrl+Shift+G" = "grid"
            ''';

          You can also use the cross-platform "Primary" modifier which maps to
          Cmd on macOS and Ctrl on Linux.
        '';
      };
      configFile = lib.mkOption {
        type = lib.types.nullOr lib.types.path;
        default = null;
        description = "Path to existing config.toml configuration file. Takes precedence over config option.";
      };
      systemd = {
        restart = lib.mkOption {
          type = lib.types.str;
          default = "on-failure";
          description = "Systemd restart policy for the Neru service.";
        };
        restartSec = lib.mkOption {
          type = lib.types.int;
          default = 5;
          description = "Seconds to wait before restarting the Neru service.";
        };
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
            + (lib.optionalString (cfg.configFile != null || cfg.config != "") " --config ${configFile}");
          Restart = cfg.systemd.restart;
          RestartSec = cfg.systemd.restartSec;
          Nice = -10;
        };
      };
    }
  );
}
