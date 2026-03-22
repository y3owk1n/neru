{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.services.neru;
  configFile = pkgs.writeScript "config.toml" cfg.config;
in
{
  options = {
    services.neru = with lib.types; {
      enable = lib.mkEnableOption "Neru keyboard navigation";
      package = lib.mkPackageOption pkgs "neru" { };
      config = lib.mkOption {
        type = types.lines;
        default = builtins.readFile ./configs/default-config.toml;
        description = "Config to use for {file} `neru/config.toml`.";
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
