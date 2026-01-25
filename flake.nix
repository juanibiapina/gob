{
  description = "Background job management for HUMANs and AGENTs";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in {
      packages = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          version = self.shortRev or self.dirtyShortRev or "dev";
        in {
          default = pkgs.buildGoModule {
            pname = "gob";
            version = version;
            src = ./.;
            vendorHash = "sha256-hDkl1HCdQdwwUdRDEQzGgbV5uZ7BAHsNHuzj2GOSkPM=";
            ldflags = [ "-s" "-w" "-X github.com/juanibiapina/gob/internal/version.Version=${version}" ];
          };
        }
      );
    };
}
