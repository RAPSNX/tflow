self:
{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.programs.tflow;
in
{
  options.programs.tflow = {
    enable = lib.mkEnableOption "tflow";
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ self.packages.${pkgs.stdenv.hostPlatform.system}.default ];
  };
}
