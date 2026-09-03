{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.services.neru;
  tomlFormat = pkgs.formats.toml {};
  configFile =
    if cfg.configFile != null
    then cfg.configFile
    else if cfg.settings != {}
	then tomlFormat.generate "config.toml" cfg.settings
    else pkgs.writeText "config.toml" cfg.config;
  effectiveEnv = {
    PATH = "/run/current-system/sw/bin:/nix/var/nix/profiles/default/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin";
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
        description = "Config to use for {file} `neru/config.toml`.";
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
        enable = lib.mkEnableOption "the launchd agent managing the Neru process" // {
          default = true;
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
          PATH = "/run/current-system/sw/bin:/nix/var/nix/profiles/default/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin";
        };
        description = ''
          Additional environment variables to set in the launchd service.
          These are merged with defaults such as a {env}`PATH`
          that includes common Nix binary directories.
          Setting {env}`PATH` here will override the default entirely.

          To extend the default PATH with additional directories:
          ```nix
          services.neru.extraEnvironment = {
            PATH = "/Users/me/.local/bin:/run/current-system/sw/bin:/nix/var/nix/profiles/default/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin";
          };
          ```
        '';
      };
    };
  };
  config = (
    lib.mkIf (cfg.enable) {
      environment.systemPackages = [ cfg.package ];

      launchd.user.agents.neru = lib.mkIf cfg.launchd.enable {
        command =
          "${cfg.package}/Applications/Neru.app/Contents/MacOS/neru launch"
          + (lib.optionalString (cfg.configFile != null || cfg.config != "") " --config ${configFile}");
        serviceConfig = {
          EnvironmentVariables = effectiveEnv;
          KeepAlive = cfg.launchd.keepAlive;
          RunAtLoad = true;
          # Neither stream is redirected. This agent is installed system-wide
          # and runs for whichever user logs in, so no per-user log path can be
          # written here at build time, and a shared one (/tmp) would put the
          # daemon's output where every local user can read it. Neru writes its
          # own rotated log to ~/Library/Logs/neru/app.log regardless; for a
          # single-user machine that also wants crash output captured, use the
          # home-manager module, which can name the user's own log directory.
          ProcessType = "Interactive";
          LimitLoadToSessionType = "Aqua";
          Nice = -10;
          ThrottleInterval = 10;
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
            services.neru: only one of the following options may be set:
              - services.neru.settings
              - services.neru.config
              - services.neru.configFile

            Please choose a single configuration source.
          '';
        }
      ];

    }
  );
}
