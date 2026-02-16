{
  description = "Navigate macOS without touching your mouse - keyboard-driven productivity at its finest";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/b976292fb39a449bcf410219e4cf0aa05a8b4d04?narHash=sha256-NmiCO/7hKv3TVIXXtEAkpGHiJzQc/5z8PT8tO+SKPZA=";
  };

  outputs =
    { self, nixpkgs, ... }:
    let
      eachSystem = nixpkgs.lib.genAttrs [
        "aarch64-darwin"
        "x86_64-darwin"
      ];

      # Update this to your latest release version
      latestVersion = "1.18.3";

      # Function to build package with specific version
      makeNeruPackage =
        pkgs: version: useZip: commitHash:
        pkgs.callPackage ./package.nix {
          inherit version useZip commitHash;
        };
    in
    {
      overlays.default = final: prev: {
        neru = makeNeruPackage final latestVersion true null;
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
          # Default: latest version from zip
          default = makeNeruPackage pkgs latestVersion true null;

          # Build from source
          source = makeNeruPackage pkgs "main" false (self.rev or self.dirtyRev or "unknown");
        }
      );

      darwinModules.default = import ./module.nix;
      homeManagerModules.default = import ./home-module.nix;
    };
}
