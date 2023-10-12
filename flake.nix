{
  description = "A Kubernetes mutating webhook that makes direct secret injection into Pods possible";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    nixpkgs-23-05.url = "github:NixOS/nixpkgs/release-23.05"; # TODO: remove once helm is fixed
    flake-parts.url = "github:hercules-ci/flake-parts";
    devenv.url = "github:cachix/devenv";
    garden.url = "github:sagikazarmark/nix-garden";
  };

  outputs = inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [
        inputs.devenv.flakeModule
      ];

      systems = [ "x86_64-linux" "x86_64-darwin" "aarch64-darwin" ];

      perSystem = { config, self', inputs', pkgs, system, ... }: rec {
        devenv.shells = {
          default = {
            languages = {
              go.enable = true;
              go.package = pkgs.go_1_21;
            };

            services = {
              vault.enable = true;
            };

            pre-commit.hooks = {
              nixpkgs-fmt.enable = true;
              yamllint.enable = true;
              hadolint.enable = true;
            };

            packages = with pkgs; [
              gnumake

              # golangci-lint
              # TODO: remove once https://github.com/NixOS/nixpkgs/pull/254878 hits unstable
              (golangci-lint.override (prev: {
                buildGoModule = pkgs.buildGo121Module;
              }))

              kind
              kubectl
              kustomize
              # kubernetes-helm
              helm-docs

              k3d

              crc

              yamllint
              hadolint
            ] ++ [
              inputs'.garden.packages.garden
              inputs'.nixpkgs-23-05.legacyPackages.kubernetes-helm
              self'.packages.licensei
            ];

            env = {
              GARDEN_DISABLE_ANALYTICS = "true";

              KUBECONFIG = "${config.devenv.shells.default.env.DEVENV_STATE}/kube/config";
              KIND_CLUSTER_NAME = "vault-secrets-webhook";

              HELM_CACHE_HOME = "${config.devenv.shells.default.env.DEVENV_STATE}/helm/cache";
              HELM_CONFIG_HOME = "${config.devenv.shells.default.env.DEVENV_STATE}/helm/config";
              HELM_DATA_HOME = "${config.devenv.shells.default.env.DEVENV_STATE}/helm/data";

              VAULT_TOKEN = "227e1cce-6bf7-30bb-2d2a-acc854318caf";
            };

            # https://github.com/cachix/devenv/issues/528#issuecomment-1556108767
            containers = pkgs.lib.mkForce { };
          };

          ci = devenv.shells.default;
        };

        packages = {
          # TODO: create flake in source repo
          licensei = pkgs.buildGoModule rec {
            pname = "licensei";
            version = "0.8.0";

            src = pkgs.fetchFromGitHub {
              owner = "goph";
              repo = "licensei";
              rev = "v${version}";
              sha256 = "sha256-Pvjmvfk0zkY2uSyLwAtzWNn5hqKImztkf8S6OhX8XoM=";
            };

            vendorSha256 = "sha256-ZIpZ2tPLHwfWiBywN00lPI1R7u7lseENIiybL3+9xG8=";

            subPackages = [ "cmd/licensei" ];

            ldflags = [
              "-w"
              "-s"
              "-X main.version=v${version}"
            ];
          };
        };
      };
    };
}
