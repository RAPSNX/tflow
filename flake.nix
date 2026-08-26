{
  description = "tflow";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      home-manager,
      ...
    }:
    let
      inherit (nixpkgs) lib;

      systems = [
        "x86_64-linux"
        "aarch64-darwin"
        "aarch64-linux"
      ];

      pkgsFor = lib.genAttrs systems (
        system:
        import nixpkgs {
          inherit system;
          config.allowUnfree = true;
        }
      );

      forAllSystems = f: lib.genAttrs systems (system: f pkgsFor.${system});

      mkTflow =
        pkgs:
        let
          version = "devel-${self.shortRev or "dirty"}";
        in
        pkgs.buildGoModule {
          pname = "tflow";
          inherit version;

          src = self;

          vendorHash = "sha256-my7yKS36TlSAUaQbz/G5drFQ6WSj9P5KQ4Hqtvvo2sE=";

          subPackages = [
            "cmd/tflow"
          ];

          ldflags = [
            "-X github.com/rapsnx/tflow/internal/buildinfo.version=${version}"
          ];
        };

      mkHomeManagerEvaluation =
        pkgs: tflow:
        home-manager.lib.homeManagerConfiguration {
          inherit pkgs;

          modules = [
            self.homeManagerModules.tflow

            {
              home = {
                username = "tflow-test";

                homeDirectory = if pkgs.stdenv.isDarwin then "/Users/tflow-test" else "/home/tflow-test";

                stateVersion = "24.05";
              };

              programs.tflow = {
                enable = true;
              };
            }
          ];
        };
    in
    {
      formatter = forAllSystems (pkgs: pkgs.nixfmt);

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gotools
            gopls
          ];
        };
      });

      packages = forAllSystems (
        pkgs:
        let
          tflow = mkTflow pkgs;
        in
        {
          default = tflow;
          inherit tflow;
        }
      );

      checks = forAllSystems (
        pkgs:
        let
          tflow = mkTflow pkgs;

          homeManagerEvaluation = mkHomeManagerEvaluation pkgs tflow;

          homeManagerStoreFile = ".local/state/tflow/store.json";

          homeManagerCheck =
            assert !(builtins.hasAttr homeManagerStoreFile homeManagerEvaluation.config.home.file);
            pkgs.runCommand "tflow-home-manager-module-evaluation" { } ''
              touch "$out"
            '';

          packageLayoutCheck = pkgs.runCommand "tflow-package-layout" { } ''
            test -x "${tflow}/bin/tflow"
            test ! -e "${tflow}/bin/cmd"
            touch "$out"
          '';
        in
        {
          default = tflow;
          home-manager-module = homeManagerCheck;
          package-layout = packageLayoutCheck;
        }
      );

      homeManagerModules = {
        default = self.homeManagerModules.tflow;
        tflow = import ./nix/home-manager.nix self;
      };
    };
}
