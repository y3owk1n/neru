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
        description = ''
          Configuration for {file} `neru/config.toml`.

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

            In case of failure, check the logs with `cat ~/Library/Logs/neru/app.log`.
            If the app fails to launch at all, check `cat /tmp/neru.err.log` for errors from the `open` command.

            For more detailed service status, run `launchctl print gui/$(id -u)/org.nix-community.home.neru`.
          '';
        };
        keepAlive = lib.mkOption {
          type = lib.types.bool;
          default = true;
          description = "Whether the launchd service should be kept alive.";
        };
      };

      systemd = {
        enable = lib.mkOption {
          type = lib.types.bool;
          default = true;
          description = ''
            Configure a systemd user service to manage the Neru process on Linux.
            You can verify the service is running correctly from your terminal.
            Run: `systemctl --user status neru`
            In case of failure, check the logs with `journalctl --user -u neru`.
          '';
        };
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
          "${cfg.package}/Applications/Neru.app/Contents/MacOS/neru"
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

    # Systemd user service for Linux
    systemd.user.services.neru = lib.mkIf (pkgs.stdenv.isLinux && cfg.systemd.enable) {
      Unit = {
        Description = "Neru keyboard navigation daemon";
        After = [ "graphical-session.target" ];
        PartOf = [ "graphical-session.target" ];
      };
      Service = {
        ExecStart = "${cfg.package}/bin/neru launch --config ${config.xdg.configHome}/neru/config.toml";
        Restart = cfg.systemd.restart;
        RestartSec = cfg.systemd.restartSec;
        Nice = "-10";
      };
      Install = {
        WantedBy = [ "graphical-session.target" ];
      };
    };
  };
}
