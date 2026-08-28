{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.services.neru;
  tomlFormat = pkgs.formats.toml {};
  defaultPath = lib.concatStringsSep ":" (
    [
      "${config.home.homeDirectory}/.nix-profile/bin"
      "/etc/profiles/per-user/${config.home.username}/bin"
      "/run/current-system/sw/bin"
      "/nix/var/nix/profiles/default/bin"
      "/usr/local/bin"
      "/usr/bin"
      "/bin"
    ]
    ++ lib.optionals pkgs.stdenv.hostPlatform.isDarwin [ "/opt/homebrew/bin" ]
  );
  effectiveEnv = {
    PATH = defaultPath;
  }
  // cfg.extraEnvironment;
in
{
  options = {
    services.neru = {
      enable = lib.mkEnableOption "Neru keyboard navigation";

      package = lib.mkPackageOption pkgs "neru" { };

      config = lib.mkOption {
        type = lib.types.lines;
        default = builtins.readFile ../configs/default-config.toml;
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
	
	  settings = lib.mkOption {
        inherit (tomlFormat) type;
        default = {};
        description = ''
          Configuration of neru in nix programming language
        '';
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

            If the app fails to launch at all, check
            `cat ~/Library/Logs/neru/daemon.err.log` for launch errors.

            For more detailed service status, run `launchctl print gui/$(id -u)/org.nix-community.home.neru`.
          '';
        };
        keepAlive = lib.mkOption {
          type = lib.types.bool;
          default = true;
          description = "Whether the launchd service should be kept alive.";
        };
      };

      extraEnvironment = lib.mkOption {
        type = lib.types.attrsOf lib.types.str;
        default = { };
        example = {
          PATH = "/run/current-system/sw/bin:/nix/var/nix/profiles/default/bin:/usr/local/bin:/usr/bin:/bin";
        };
        description = ''
          Additional environment variables to set in the launchd (macOS) or systemd (Linux) service.
          These are merged with defaults such as a {env}`PATH`
          that includes common Nix binary directories and the user's Nix profile.
          Setting {env}`PATH` here will override the default entirely.

          To extend the default PATH with additional directories:
          ```nix
          services.neru.extraEnvironment = {
            PATH = "/Users/me/.cargo/bin:/Users/me/.nix-profile/bin:/etc/profiles/per-user/me/bin:/run/current-system/sw/bin:/nix/var/nix/profiles/default/bin:/usr/local/bin:/usr/bin:/bin";
          };
          ```
        '';
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
      if cfg.configFile != null
      then {source = cfg.configFile;}
      else if cfg.settings != {}
      then {
        source = tomlFormat.generate "config.toml" cfg.settings;
      }
      else {text = cfg.config;};


    # Launch agent for macOS
    launchd.agents.neru = lib.mkIf pkgs.stdenv.hostPlatform.isDarwin {
      enable = cfg.launchd.enable;
      config = {
        ProgramArguments = [
          "${cfg.package}/Applications/Neru.app/Contents/MacOS/neru"
          "launch"
          "--config"
          "${config.xdg.configHome}/neru/config.toml"
        ];
        EnvironmentVariables = effectiveEnv;
        RunAtLoad = true;
        KeepAlive = cfg.launchd.keepAlive;
        # Standard output is left alone: Neru's own rotated log file already
        # holds every log line. Standard error goes beside it, in the user's own
        # log directory rather than a shared one, for a crash or a failure
        # raised before that file is open.
        #
        # launchd creates the file but never the directory holding it, and the
        # directory is created by Neru itself on its first run — so on a machine
        # that has never run Neru, the very first spawn has nowhere to redirect
        # to and launchd drops its stderr. Every spawn after that one is
        # captured. (`neru services install`, which writes its own agent, closes
        # that gap by creating the directory before bootstrapping.)
        StandardErrorPath = "${config.home.homeDirectory}/Library/Logs/neru/daemon.err.log";
        ProcessType = "Interactive";
        LimitLoadToSessionType = "Aqua";
        Nice = -10;
        ThrottleInterval = 10;
      };
    };

    # Systemd user service for Linux
    systemd.user.services.neru = lib.mkIf (pkgs.stdenv.hostPlatform.isLinux && cfg.systemd.enable) {
      Unit = {
        Description = "Neru keyboard navigation daemon";
        After = [ "graphical-session.target" ];
        PartOf = [ "graphical-session.target" ];
      };
      Service = {
        ExecStart = "${cfg.package}/bin/neru launch --config ${config.xdg.configHome}/neru/config.toml";
        Environment = lib.mapAttrsToList (k: v: "${k}=${v}") effectiveEnv;
        Restart = cfg.systemd.restart;
        RestartSec = cfg.systemd.restartSec;
        Nice = "-10";
      };
      Install = {
        WantedBy = [ "graphical-session.target" ];
      };
    };

	assertions = [
      # Fail if user set more than one configuration source
      {
        assertion =
          (lib.count (x: x) [
            (cfg.settings != {})
            (cfg.configFile != null)
            (cfg.config != builtins.readFile ../configs/default-config.toml)
          ])
          <= 1;

        message = ''
          programs.neru: only one of the following options may be set:
            - programs.neru.settings
            - programs.neru.config
            - programs.neru.configFile

          Please choose a single configuration source.
        '';
      }
    ];

  };
}
