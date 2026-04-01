{
  description = "A Nix-flake-based Go development environment for mutating-registry-webhook";

  inputs = {
    nixpkgs.url = "https://flakehub.com/f/NixOS/nixpkgs/0.2511";
    nixpkgs-unstable.url = "https://flakehub.com/f/NixOS/nixpkgs/0.1";
  };

  outputs = { self, nixpkgs, nixpkgs-unstable }:
    let
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      forEachSupportedSystem = f:
        nixpkgs.lib.genAttrs supportedSystems (system:
          let
            pkgs = import nixpkgs {
              inherit system;
            };
            pkgs-unstable = import nixpkgs-unstable {
              inherit system;
            };
          in
          f { pkgs = pkgs; pkgs-unstable = pkgs-unstable; system = system; }
        );
    in
    {
      devShells = forEachSupportedSystem ({ pkgs, pkgs-unstable, system }:
        let
          stablePackages = with pkgs; [
            ginkgo
            go_1_26
            gotools
            kind
            kubectl
            kubernetes-helm
            kustomize_4
            setup-envtest
          ];
          unstablePackages = with pkgs-unstable; [
            golangci-lint
          ];
        in
        {
          default = pkgs.mkShell {
            packages = stablePackages ++ unstablePackages;
          };
        }
      );
    };
}
