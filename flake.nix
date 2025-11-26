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
    in
    {
      overlays.default = final: prev: {
        neru = final.callPackage ./package.nix { };
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
          default = pkgs.neru;
          neru = pkgs.neru;
        }
      );

      devShells = eachSystem (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              go
              gopls
              gotools
              gofumpt
              golangci-lint
              golines
              just # just a command runner like make
              clang-tools
            ];
          };
        }
      );

      darwinModules.default = import ./module.nix;
    };
}
