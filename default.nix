{stdenv, lib, goPackages, git, runCommand}:
let
	removeReferences = [ goPackages.go ];
	removeExpr = refs: lib.flip lib.concatMapStrings refs (ref: ''
		| sed "s,${ref},$(echo "${ref}" | sed "s,$NIX_STORE/[^-]*,$NIX_STORE/eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee,"),g" \
	'');
in
goPackages.buildGoPackage rec {
	name = "focus";

	goPackagePath = "github.com/mstone/focus";

	src = runCommand "focus-git" {
		nativeBuildInputs = [ git ];
		preferLocalBuild = true;
		localRev = builtins.readFile ./.git/refs/heads/master;
	} ''
		mkdir -p $out && git -C ${toString ./.} archive --format=tar $localRev | tar xf - -C $out
		git -C ${toString ./.} submodule foreach 'git -C ${toString ./.}/$path archive HEAD | tar xf - -C $out/$path'
		cp -r ${toString ./.}/.git $out/.git
	'';

	buildInputs = [
	] ++ (with goPackages; [
		log15
		negroni
		websocket
		go-sqlite3
		render
		logfmt
		go-bindata-html-template
		sqlx
		juju-errors
		gopherjs
		gopherjs.bin
		go-bindata.bin
		esc.bin
		goconvey.bin
	]);

	postConfigure = ''
		(cd go/src/${goPackagePath}; go generate)
	'';

	excludedPackages = "kitchen-sink";

	preFixup = ''
		while read file; do
			cat $file ${removeExpr removeReferences} > $file.tmp
			mv $file.tmp $file
			chmod +x $file
		done < <(find $out -type f 2>/dev/null)
	'';

	shellHook = ''
		if [ -z "${builtins.getEnv "GOPATH"}" ]; then
			echo "WARNING: the focus dev-shell requires that the focus source code working copy be placed or mounted in a standard GOPATH workspace, not accessed via symlink"
			echo "ERROR: GOPATH not set; exiting"
			exit 1
		fi
	'';
}