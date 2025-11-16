{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/b976292fb39a449bcf410219e4cf0aa05a8b4d04?narHash=sha256-NmiCO/7hKv3TVIXXtEAkpGHiJzQc/5z8PT8tO+SKPZA=";
  };

  outputs =
    { nixpkgs, ... }:
    let
      eachSystem = nixpkgs.lib.genAttrs [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-darwin"
        "x86_64-linux"
      ];
    in
    {
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

    };
}
