{ _pkgs ? import <nixpkgs> {} }: let
env = rec {
  pkgs = import (_pkgs.fetchFromGitHub {
    owner = "mstone";
    repo = "nixpkgs";
    rev = "76d85864d18f0fb59115f36a6842e10b71d3a1cf";
    sha256 = "0fwczhywg7bxpw5zpapppp93bv769vfx54hpqss4giclz80w0ipk";
    }) {};

  buildGoPackage = pkgs.buildGo18Package;

  gopherjs = (pkgs.lib.callPackageWith pkgs ./nix/gopherjs.nix {});

  go-bindata = (pkgs.lib.callPackageWith pkgs ./nix/go-bindata.nix {});

  esc = (pkgs.lib.callPackageWith pkgs ./nix/esc.nix {});

  bin = (pkgs.lib.callPackageWith (pkgs // {
    inherit gopherjs go-bindata esc;
  }) ./default.nix {}).bin;
}; in env
