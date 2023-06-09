{
  description = "A Kubernetes mutating webhook that makes direct secret injection into Pods possible";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
    devenv.url = "github:cachix/devenv";
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
            };

            services = {
              vault.enable = true;
            };

            packages = with pkgs; [
              gnumake

              golangci-lint

              kind
              kubectl
              kustomize
              kubernetes-helm
              helm-docs
            ] ++ [
              self'.packages.licensei
            ];

            scripts = {
              versions.exec = ''
                go version
                golangci-lint version
                kind version
                kubectl version --client
                echo kustomize $(kustomize version --short)
                echo helm $(helm version --short)
              '';
            };

            enterShell = ''
              # Vault
              export VAULT_ADDR=http://127.0.0.1:8200
              export VAULT_TOKEN=227e1cce-6bf7-30bb-2d2a-acc854318caf

              versions
            '';

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
