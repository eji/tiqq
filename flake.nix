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
          goArchives = {
            aarch64-darwin = {
              name = "darwin-arm64";
              hash = "sha256-kEk7O71eEPkdEhUxmL8ZlP11Y5m0/sk7SbDG4qze6z4=";
            };
            x86_64-darwin = {
              name = "darwin-amd64";
              hash = "sha256-0zFOJUluQ4HXGlxR0pB+evZV0Zn2eAtUnwFb2F/vSYY=";
            };
            aarch64-linux = {
              name = "linux-arm64";
              hash = "sha256-UXmNLELQ4cbtf9n0hyi0GTq6yeiq1tusL+lqgfWQm9o=";
            };
            x86_64-linux = {
              name = "linux-amd64";
              hash = "sha256-Z1wmxEnLsY/CS3RlDeHqu65uFvZDJv2FooP7O1goBoU=";
            };
          };
          goArchive = goArchives.${system};
          go_1_27 = pkgs.stdenvNoCC.mkDerivation {
            pname = "go";
            version = "1.27.0";
            src = pkgs.fetchurl {
              url = "https://go.dev/dl/go1.27.0.${goArchive.name}.tar.gz";
              inherit (goArchive) hash;
            };
            sourceRoot = "go";
            dontBuild = true;
            installPhase = ''
              runHook preInstall
              mkdir -p "$out"
              cp -R . "$out"
              runHook postInstall
            '';
          };
        in
        {
          default = pkgs.mkShell {
            packages = [
              go_1_27
              pkgs.gnumake
              pkgs.gitleaks
              pkgs.pre-commit
            ];

            shellHook = ''
              expected="go version go1.27.0"
              actual="$(go version)"
              case "$actual" in
                "$expected"*) ;;
                *)
                  echo "tiqq: expected Go 1.27.0, got: $actual" >&2
                  exit 1
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
