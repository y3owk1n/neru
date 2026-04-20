{
  description = "Navigate macOS without touching your mouse - keyboard-driven productivity at its finest";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs, ... }:
    let
      eachSystem = nixpkgs.lib.genAttrs [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-darwin"
        "x86_64-linux"
      ];

      # Update this to your latest release version
      latestVersion = "1.32.0";

      # Function to build package with specific version
      makeNeruPackage =
        pkgs: version: useZip: commitHash:
        pkgs.callPackage ./package.nix {
          inherit version useZip commitHash;
        };
    in
    {
      overlays.default = final: prev: {
        neru =
          if final.stdenv.hostPlatform.isDarwin then
            makeNeruPackage final latestVersion true null
          else
            makeNeruPackage final "main" false (self.rev or self.dirtyRev or "unknown");
        neru-source = makeNeruPackage final "main" false (self.rev or self.dirtyRev or "unknown");
      };

      # Packages output using the overlay
      packages = eachSystem (
        system:
        let
          pkgs = import nixpkgs {
            inherit system;
            overlays = [ self.overlays.default ];
          };
        in
        {
          # Default: latest version from zip on macOS, source build elsewhere.
          default =
            if pkgs.stdenv.hostPlatform.isDarwin then
              makeNeruPackage pkgs latestVersion true null
            else
              makeNeruPackage pkgs "main" false (self.rev or self.dirtyRev or "unknown");

          # Build from source
          source = makeNeruPackage pkgs "main" false (self.rev or self.dirtyRev or "unknown");
        }
      );

      darwinModules.default = import ./darwin-module.nix;
      nixosModules.default = import ./nixos-module.nix;
      homeManagerModules.default = import ./home-module.nix;
    };
}
