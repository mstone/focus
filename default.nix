{stdenv, lib, buildGoPackage, git, runCommand, go2nix, gopherjs, go-bindata, esc}:
buildGoPackage rec {
    name = "focus";
    version = "git${stdenv.lib.strings.substring 0 7 rev}";
    rev = (builtins.readFile (./.git + ("/" + (builtins.replaceStrings ["\n"] [""] (stdenv.lib.strings.removePrefix "ref: " (builtins.readFile ./.git/HEAD))))));

    goPackagePath = "github.com/mstone/focus";
    goDeps = ./godeps.nix;

    buildInputs = [ go2nix gopherjs gopherjs.bin go-bindata.bin esc.bin ];

    src = runCommand "focus-git" {
        nativeBuildInputs = [ git ];
        preferLocalBuild = true;
        localRev = rev;
    } ''
        mkdir -p $out && git -C ${toString ./.} archive --format=tar $localRev | tar xf - -C $out
        git -C ${toString ./.} submodule foreach 'git -C ${toString ./.}/$path archive HEAD | tar xf - -C $out/$path'
        cp -r ${toString ./.}/.git $out/.git
    '';

    postConfigure = ''
        (export GOPATH="`pwd`/go:$GOPATH"; cd go/src/${goPackagePath}; go generate)
    '';

    postInstall = stdenv.lib.optionalString stdenv.isDarwin ''
        # Fix rpaths to avoid derivation output cycles
        for file in $(grep -rl $out/lib $bin); do
            install_name_tool -delete_rpath $out/lib -add_rpath $bin $file
        done
    '';

    excludedPackages = "kitchen-sink";

    shellHook = ''
        if [ -z "${builtins.getEnv "GOPATH"}" ]; then
            echo "WARNING: the focus dev-shell requires that the focus source code working copy be placed or mounted in a standard GOPATH workspace, not accessed via symlink"
            echo "ERROR: GOPATH not set; exiting"
            exit 1
        fi
    '';
}
