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
		mkdir -p gopath/{src,pkg,bin};
		mkdir -p $(dirname gopath/src/${goPackagePath});
		if [ ! -L gopath/src/${goPackagePath} ]; then
			ln -sf $(pwd) gopath/src/${goPackagePath};
		fi
		export GOPATH=$(pwd)/gopath:$GOPATH;
	'';
}