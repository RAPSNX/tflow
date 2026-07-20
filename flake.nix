{
  description = "tflow";

  inputs = {
    flake-utils.url = "github:numtide/flake-utils";
    home-manager.url = "github:nix-community/home-manager";
    home-manager.inputs.nixpkgs.follows = "nixpkgs";
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    {
      self,
      flake-utils,
      home-manager,
      nixpkgs,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
        tflow = pkgs.buildGoModule {
          pname = "tflow";
          version = "0.1.0";
          src = self;
          vendorHash = "sha256-UORW9eGwCwt/rakuC10j3PEFCmlobyJk09jSIqVHZo8=";
          subPackages = [ "cmd/tflow" ];
        };
        homeManagerEvaluation = home-manager.lib.homeManagerConfiguration {
          inherit pkgs;
          modules = [
            (import ./nix/home-manager.nix)
            {
              home.username = "tflow-test";
              home.homeDirectory = "/home/tflow-test";
              home.stateVersion = "24.05";
              programs.tflow = {
                enable = true;
                package = tflow;
              };
            }
          ];
        };
        homeManagerStoreFile = ".local/state/tflow/store.json";
        homeManagerCheck =
          assert !(builtins.hasAttr homeManagerStoreFile homeManagerEvaluation.config.home.file);
          pkgs.runCommand "tflow-home-manager-module-evaluation" { } ''
            touch $out
          '';
        packageLayoutCheck = pkgs.runCommand "tflow-package-layout" { } ''
          test -x "${tflow}/bin/tflow"
          test ! -e "${tflow}/bin/cmd"
          touch $out
        '';
      in
      {
        checks.default = tflow;
        checks.home-manager-module = homeManagerCheck;
        checks.package-layout = packageLayoutCheck;
        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.gotools
          ];
        };
        packages.default = tflow;
        packages.tflow = tflow;
      }
    )
    // {
      homeManagerModules.default = import ./nix/home-manager.nix;
    };
}
