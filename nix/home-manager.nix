{ config, lib, pkgs, ... }:

let
  cfg = config.programs.tflow;
in
{
  options.programs.tflow = {
    enable = lib.mkEnableOption "tflow";

    package = lib.mkOption {
      type = lib.types.package;
      default = pkgs.writeShellScriptBin "tflow" ''
        echo "programs.tflow.package is not set" >&2
        exit 1
      '';
      description = "Package to install for tflow.";
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ];
  };
}
