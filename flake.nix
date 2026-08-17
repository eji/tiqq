{
  description = "tiqq development environment";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs = { nixpkgs, ... }:
    let
      systems = [
        "aarch64-darwin"
        "x86_64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      devShells = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.mkShell {
            packages = [
              pkgs.go_1_27
              pkgs.gnumake
              pkgs.gitleaks
              pkgs.pre-commit
            ];

            shellHook = ''
              expected="go version go1.27rc2"
              actual="$(go version)"
              case "$actual" in
                "$expected"*) ;;
                *)
                  echo "tiqq: expected Go 1.27 RC2, got: $actual" >&2
                  return 1
                  ;;
              esac

              hook_path="$(git rev-parse --git-path hooks/pre-commit 2>/dev/null || true)"
              if [ -n "$hook_path" ] && [ ! -e "$hook_path" ]; then
                pre-commit install
              fi
            '';
          };
        });
    };
}
