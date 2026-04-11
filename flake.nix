{
  description = "git-different";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    pre-commit-hooks = {
      url = "github:cachix/pre-commit-hooks.nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, pre-commit-hooks }:
    let
      forAllSystems = fn:
        nixpkgs.lib.genAttrs [
          "x86_64-linux"
          "aarch64-linux"
          "x86_64-darwin"
          "aarch64-darwin"
        ] (system: fn system nixpkgs.legacyPackages.${system});
    in
    {
      checks = forAllSystems (system: pkgs: {
        pre-commit-check = pre-commit-hooks.lib.${system}.run {
          src = ./.;
          package = pkgs.prek;
          hooks = {
            gofmt.enable = true;
            govet.enable = true;
            golangci-lint.enable = true;
          };
        };
      });

      devShells = forAllSystems (system: pkgs: {
        default = pkgs.mkShell {
          inherit (self.checks.${system}.pre-commit-check) shellHook;

          packages = with pkgs; [
            go
            gopls
            golangci-lint
            betteralign
          ];
        };
      });
    };
}
