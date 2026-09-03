{
  description = "Nix Development Shell for gh-runner";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShells.default = pkgs.mkShell {
          name = "gh-runner-dev-shell";

          packages = with pkgs; [
            # Go and Go Tools
            go
            gopls
            gotools
            golangci-lint

            # Database / Code Generation
            sqlc
            goose
            protobuf
            protoc-gen-go
            protoc-gen-go-grpc
            buf
            air

            # Containerization & Orchestrators
            docker
            docker-compose

            # VCS & Command-line Tools
            git
            curl
            gh

            # Frontend & Web UI (M8)
            nodejs
            pnpm

            # Linting & Formatting
            golangci-lint
            hadolint
            oxlint
            shellcheck
            shfmt
          ];

          shellHook = ''
            echo "======================================================="
            echo "   🛡️ gh-runner Nix Development Shell Loaded 🛡️"
            echo "   Go:              $(go version | awk '{print $3}')"
            echo "   Node:            $(node --version)"
            echo "   pnpm:            v$(pnpm --version)"
            echo "   golangci-lint:   $(golangci-lint --version | awk '{print $4}')"
            echo "   hadolint:        $(hadolint --version | head -n1 | awk '{print $4}')"
            echo "   oxlint:          $(oxlint --version | awk '{print $2}')"
            echo "   shellcheck:      $(shellcheck --version | grep 'version:' | awk '{print $2}')"
            echo "   shfmt:           $(shfmt --version)"
            echo "======================================================="
          '';
        };
      }
    );
}
