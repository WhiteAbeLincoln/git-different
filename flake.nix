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

      packages = forAllSystems (system: pkgs: {
        default = pkgs.buildGoModule {
          pname = "git-different";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-X22jl+ech8IsGTfD6u0vW7EBYjZlDtINKvpHhM4vrkc=";
          nativeCheckInputs = [ pkgs.git ];
          preCheck = ''
            export HOME=$TMPDIR
            git config --global user.email "test@test.com"
            git config --global user.name "Test"
          '';
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
            svu
            go-task
          ];
        };
      });
    };
}
