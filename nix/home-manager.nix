{ config, lib, pkgs, ... }:

let
  cfg = config.programs.tflow;

  yamlString = value: builtins.toJSON value;

  projectFileName = name:
    let
      normalized = lib.replaceStrings [ " " "/" "_" "." ] [ "-" "-" "-" "-" ] (lib.toLower name);
    in
    "${normalized}.yaml";

  renderProject = name: project:
    let
      projectName = project.name or name;
    in
    lib.concatStrings [
      "name: ${yamlString projectName}\n"
      (lib.optionalString (project.workdir != null) "workdir: ${yamlString project.workdir}\n")
      (lib.optionalString project.protect "protect: true\n")
      (lib.optionalString (project.agentBinary != null) "agent-binary: ${yamlString project.agentBinary}\n")
      (lib.optionalString (project.cluster.path != null || project.cluster.connectionCmd != null) (
        "cluster:\n"
        + lib.optionalString (project.cluster.path != null) "  path: ${yamlString project.cluster.path}\n"
        + lib.optionalString (project.cluster.connectionCmd != null) "  connection-cmd: ${yamlString project.cluster.connectionCmd}\n"
      ))
    ];

  projectsDirDefault = "${config.xdg.configHome}/tflow/projects";
  configPath = "tflow/config.yaml";
  projectFiles =
    lib.mapAttrs'
      (
        name: project:
        lib.nameValuePair
          (lib.removePrefix "${config.home.homeDirectory}/" "${cfg.settings.projectsDir}/${projectFileName name}")
          { text = renderProject name project; }
      )
      cfg.settings.projects;

  renderConfig = ''
    projects-dir: ${yamlString cfg.settings.projectsDir}
    theme: ${yamlString cfg.settings.theme}
  ''
  + lib.optionalString (cfg.settings.colors != { }) (
    "colors:\n"
    + lib.concatStrings (
      lib.mapAttrsToList (
        key: value:
        lib.optionalString (value != null) "  ${key}: ${yamlString value}\n"
      ) cfg.settings.colors
    )
  );
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
      projectsDir = lib.mkOption {
        type = lib.types.str;
        default = projectsDirDefault;
        description = "Absolute path to the project YAML directory written into tflow config.yaml.";
      };

      theme = lib.mkOption {
        type = lib.types.enum [ "catppuccin" "forest" ];
        default = "catppuccin";
        description = "Theme name written into tflow config.yaml.";
      };

      colors = lib.mkOption {
        type = lib.types.attrsOf (lib.types.nullOr lib.types.str);
        default = { };
        description = "Optional color overrides for config.yaml.";
      };

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
        description = "Project YAMLs to render for tflow.";
      };
    };
  };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = lib.hasPrefix "${config.home.homeDirectory}/" cfg.settings.projectsDir;
        message = "programs.tflow.settings.projectsDir must live under the home directory when managed by Home Manager.";
      }
    ];

    home.packages = [ cfg.package ];

    xdg.configFile.${configPath}.text = renderConfig;
    home.file = projectFiles;
  };
}
