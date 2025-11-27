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
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ];

    # Generate config file - either from text or source file
    xdg.configFile."neru/config.toml" =
      if cfg.configFile != null then { source = cfg.configFile; } else { text = cfg.config; };

    # Launch agent for macOS
    launchd.agents.neru = lib.mkIf pkgs.stdenv.isDarwin {
      enable = true;
      config = {
        ProgramArguments = [
          "${cfg.package}/Applications/Neru.app/Contents/MacOS/Neru"
          "launch"
          "--config"
          "${config.xdg.configHome}/neru/config.toml"
        ];
        RunAtLoad = true;
        KeepAlive = true;
      };
    };
  };
}
