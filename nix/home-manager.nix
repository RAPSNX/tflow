{ config, lib, pkgs, ... }:

let
  cfg = config.programs.tflow;
  storePath = "${config.xdg.stateHome}/tflow/store.json";
  storeFile = lib.removePrefix "${config.home.homeDirectory}/" storePath;

  projectEntries = lib.mapAttrsToList (
    name: project:
    let
      projectName = project.name or name;
      cluster =
        lib.filterAttrs (_: value: value != null) {
          path = project.cluster.path;
          connection_cmd = project.cluster.connectionCmd;
        };
    in
    {
      name = projectName;
      value =
        lib.filterAttrs (_: value: value != null) {
          workdir = project.workdir;
          agent_binary = project.agentBinary;
        }
        // lib.optionalAttrs project.protect { protect = true; }
        // lib.optionalAttrs (cluster != { }) { inherit cluster; };
    }
  ) cfg.settings.projects;

  renderStore = builtins.toJSON {
    project_order = map (entry: entry.name) projectEntries;
    projects = builtins.listToAttrs projectEntries;
    session_projects = { };
    session_types = { };
  };
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
      defaultText = lib.literalExpression "inputs.tflow.packages.${pkgs.system}.default";
      description = "Package to install for tflow.";
    };

    settings = {
      projects = lib.mkOption {
        default = { };
        type = lib.types.attrsOf (
          lib.types.submodule {
            options = {
              name = lib.mkOption {
                type = lib.types.nullOr lib.types.str;
                default = null;
              };
              workdir = lib.mkOption {
                type = lib.types.nullOr lib.types.str;
                default = null;
              };
              protect = lib.mkOption {
                type = lib.types.bool;
                default = false;
              };
              agentBinary = lib.mkOption {
                type = lib.types.nullOr lib.types.str;
                default = null;
              };
              cluster = {
                path = lib.mkOption {
                  type = lib.types.nullOr lib.types.str;
                  default = null;
                };
                connectionCmd = lib.mkOption {
                  type = lib.types.nullOr lib.types.str;
                  default = null;
                };
              };
            };
          }
        );
        description = "Project metadata to render into tflow store.json.";
      };
    };
  };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = lib.hasPrefix "${config.home.homeDirectory}/" config.xdg.stateHome;
        message = "programs.tflow requires xdg.stateHome to live under the home directory when managed by Home Manager.";
      }
    ];

    home.packages = [ cfg.package ];
    home.file.${storeFile}.text = renderStore;
  };
}
