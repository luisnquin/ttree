{
  description = "A Nix-flake-based Go development environment for ttree";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = {
    self,
    nixpkgs,
  }: let
    supportedSystems = ["x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin"];
    forEachSupportedSystem = f:
      nixpkgs.lib.genAttrs supportedSystems (system:
        f {
          pkgs = import nixpkgs {inherit system;};
        });
  in {
    devShells = forEachSupportedSystem ({pkgs}: {
      default = pkgs.mkShell {
        packages = with pkgs; [
          go
          gopls
          gotools
          go-tools
          sqlite
          gcc
        ];

        env = {
          CGO_ENABLED = "1";
        };
      };
    });

    packages = forEachSupportedSystem ({pkgs}: {
      default = pkgs.buildGoModule {
        pname = "ttree";
        version = "0.1.0";
        src = ./.;

        vendorHash = "sha256-6TtiNXl4xno4g9zb0jFWyu0ZEqCk+omi91B1JIm/KPU=";
        env = {
          CGO_ENABLED = "1";
        };
        meta = with pkgs.lib; {
          description = "Terminal TUI application in Go for managing hierarchical tasks";
          license = licenses.mit; # Assuming MIT, change if needed
          maintainers = [];
        };
      };
    });
  };
}
