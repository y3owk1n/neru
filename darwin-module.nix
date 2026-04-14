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
        description = "Config to use for {file} `neru/config.toml`.";
      };
      configFile = lib.mkOption {
        type = lib.types.nullOr lib.types.path;
        default = null;
        description = "Path to existing config.toml configuration file. Takes precedence over config option.";
      };
    };
  };
  config = (
    lib.mkIf (cfg.enable) {
      environment.systemPackages = [ cfg.package ];

      # Quit the running Neru app before the launchd agent is restarted.
      # When launched via `open -W -a`, launchd only manages the `open` wrapper;
      # the actual Neru process must be terminated explicitly so the new version
      # starts after `darwin-rebuild switch`.
      #
      # nix-darwin runs all activationScripts before reloading launchd agents,
      # so this is guaranteed to execute before the agent plist is updated.
      system.activationScripts.preNeruRestart.text = ''
        if /usr/bin/pgrep -xq neru 2>/dev/null; then
          /usr/bin/osascript -e 'tell application "Neru" to quit' 2>/dev/null || true
          sleep 1
          /usr/bin/pkill -x neru 2>/dev/null || true
        fi
      '';

      launchd.user.agents.neru = {
        command =
          "/usr/bin/open -W -a ${cfg.package}/Applications/Neru.app --args launch"
          + (lib.optionalString (cfg.config != "") " --config ${configFile}");
        serviceConfig = {
          KeepAlive = true;
          RunAtLoad = true;
          ProcessType = "Interactive";
          LimitLoadToSessionType = "Aqua";
          Nice = -10;
          ThrottleInterval = 10;
        };
      };
    }
  );
}
