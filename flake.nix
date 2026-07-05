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
      in
      {
        checks.default = tflow;
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
