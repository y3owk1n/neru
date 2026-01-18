{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.services.neru;
in
{
  options = {
    services.neru = {
      enable = lib.mkEnableOption "Neru keyboard navigation";

      package = lib.mkPackageOption pkgs "neru" { };

      config = lib.mkOption {
        type = lib.types.lines;
        default = builtins.readFile ./configs/default-config.toml;
        description = "Configuration for {file} `neru/config.toml`.";
      };

      configFile = lib.mkOption {
        type = lib.types.nullOr lib.types.path;
        default = null;
        description = "Path to existing config.toml configuration file. Takes precedence over config option.";
      };

      launchd = {
        enable = lib.mkOption {
          type = lib.types.bool;
          default = true;
          description = ''
            Configure the launchd agent to manage the Neru process.

            The first time this is enabled, macOS will prompt you to allow this background
            item in System Settings.

            You can verify the service is running correctly from your terminal.
            Run: `launchctl list | grep neru`

            - A running process will show a Process ID (PID) and a status of 0, for example:
            	`12345	0	org.nix-community.home.neru`

            - If the service has crashed or failed to start, the PID will be a dash and the
            	status will be a non-zero number, for example:
            	`-	1	org.nix-community.home.neru`

            In case of failure, check the logs with `cat /tmp/neru.err.log`.

            For more detailed service status, run `launchctl print gui/$(id -u)/org.nix-community.home.neru`.
          '';
        };
        keepAlive = lib.mkOption {
          type = lib.types.bool;
          default = true;
          description = "Whether the launchd service should be kept alive.";
        };
      };
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ];

    # Generate config file - either from text or source file
    xdg.configFile."neru/config.toml" =
      if cfg.configFile != null then { source = cfg.configFile; } else { text = cfg.config; };

    # Launch agent for macOS
    launchd.agents.neru = lib.mkIf pkgs.stdenv.isDarwin {
      enable = cfg.launchd.enable;
      config = {
        ProgramArguments = [
          "${cfg.package}/Applications/Neru.app/Contents/MacOS/Neru"
          "launch"
          "--config"
          "${config.xdg.configHome}/neru/config.toml"
        ];
        RunAtLoad = true;
        KeepAlive = cfg.launchd.keepAlive;
        StandardOutPath = "/tmp/neru.log";
        StandardErrorPath = "/tmp/neru.err.log";
        ProcessType = "Interactive";
        LimitLoadToSessionType = "Aqua";
        Nice = -10;
        ThrottleInterval = 10;
      };
    };
  };
}
