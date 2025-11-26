{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.neru;
  configFile = pkgs.writeScript "neru.toml" cfg.config;
in
{
  options = {
    neru = with lib.types; {
      enable = lib.mkEnableOption "Neru keyboard navigation";
      package = lib.mkPackageOption pkgs "neru" { };
      config = lib.mkOption {
        type = types.lines;
        default = ''
          [general]
          excluded_apps = []
          accessibility_check_on_start = true
          restore_cursor_position = false

          [hotkeys]
          "Cmd+Shift+Space" = "hints"
          "Cmd+Shift+G" = "grid"
          "Cmd+Shift+S" = "action scroll"

          [hints]
          enabled = true
          hint_characters = "asdfghjkl"
          font_size = 12
          font_family = "SF Mono"
          border_radius = 4
          padding = 4
          border_width = 1
          opacity = 0.95
          background_color = "#FFD700"
          text_color = "#000000"
          matched_text_color = "#737373"
          border_color = "#000000"
          include_menubar_hints = false
          additional_menubar_hints_targets = [
              "com.apple.TextInputMenuAgent",
              "com.apple.controlcenter",
              "com.apple.systemuiserver",
          ]
          include_dock_hints = false
          include_nc_hints = false
          clickable_roles = [
              "AXButton",
              "AXComboBox",
              "AXCheckBox",
              "AXRadioButton",
              "AXLink",
              "AXPopUpButton",
              "AXTextField",
              "AXSlider",
              "AXTabButton",
              "AXSwitch",
              "AXDisclosureTriangle",
              "AXTextArea",
              "AXMenuButton",
              "AXMenuItem",
              "AXCell",
              "AXRow",
          ]
          ignore_clickable_check = false

          [hints.additional_ax_support]
          enable = false
          additional_electron_bundles = []
          additional_chromium_bundles = []
          additional_firefox_bundles = []

          [grid]
          enabled = true

          characters = "abcdefghijklmnpqrstuvwxyz"
          sublayer_keys = "abcdefghijklmnpqrstuvwxyz"
          font_size = 12
          font_family = "SF Mono"
          opacity = 0.7
          border_width = 1
          background_color = "#abe9b3"
          text_color = "#000000"
          matched_text_color = "#f8bd96"
          matched_background_color = "#f8bd96"
          matched_border_color = "#f8bd96"
          border_color = "#abe9b3"
          live_match_update = true
          hide_unmatched = true

          [scroll]
          scroll_step = 50
          scroll_step_half = 500
          scroll_step_full = 1000000
          highlight_scroll_area = true
          highlight_color = "#FF0000"
          highlight_width = 2

          [action]
          highlight_color = "#00FF00"
          highlight_width = 3
          left_click_key = "l"
          right_click_key = "r"
          middle_click_key = "m"
          mouse_down_key = "i"
          mouse_up_key = "u"

          [smooth_cursor]
          move_mouse_enabled = false
          steps = 10
          delay = 1

          [metrics]
          enabled = false

          [logging]
          log_level = "info"
          log_file = ""
          structured_logging = true
          disable_file_logging = false
          max_file_size = 10
          max_backups = 5
          max_age = 30
        '';
        description = "Config to use for {file} `neru.toml`.";
      };
    };
  };
  config = (
    lib.mkIf (cfg.enable) {
      environment.systemPackages = [ cfg.package ];
      launchd.user.agents.neru = {
        command =
          "${cfg.package}/Applications/Neru.app/Contents/MacOS/Neru launch"
          + (lib.optionalString (cfg.config != "") " --config ${configFile}");
        serviceConfig = {
          KeepAlive = false;
          RunAtLoad = true;
        };
      };
    }
  );
}
