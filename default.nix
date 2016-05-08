{stdenv, goPackages, git, runCommand, makeWrapper}:
goPackages.buildGoPackage rec {
	name = "focus";

	goPackagePath = "github.com/mstone/focus";

	src = runCommand "focus-git" {
		nativeBuildInputs = [ git ];
		preferLocalBuild = true;
		localRev = builtins.readFile ./.git/refs/heads/master;
	} ''
		mkdir -p $out && git -C ${toString ./.} archive --format=tar $localRev | tar xf - -C $out
		cp -r ${toString ./.}/.git $out/.git
	'';

	buildInputs = [
		makeWrapper
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

	postInstall = ''
		for f in $(find $bin/bin -type f); do
			wrapProgram $f \
				--prefix PATH : ${git}/bin \
				--set FOCUS_SRC ${src};
		done;
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