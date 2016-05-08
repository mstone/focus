{ pkgs ? import <nixpkgs> {} }:
let
	buildGoPackage = pkgs.go16Packages.buildGoPackage;

	buildFromGitHub = pkgs.go16Packages.buildFromGitHub;

	goPackages = pkgs.go16Packages.override rec {
		overrides = (pkgs: rec {
			negroni = buildFromGitHub {
				rev = "1dd3ab0ff59e13f5b73dcbf70703710aebe50d2f";
				owner = "codegangsta";
				repo = "negroni";
				sha256 = "0xyng3gr3xk2sld0mni9may3l1nf1kmw41hm95xdwxfbmjbd9nng";
			};
			render = buildFromGitHub {
				rev = "198ad4d8b8a4612176b804ca10555b222a086b40";
				owner = "unrolled";
				repo = "render";
				sha256 = "0vj9a5yfsfpcmyfipm3jyl82s2spsxl27rk4fkjh76zqb6820ikg";
			};
			logfmt = buildFromGitHub {
				rev = "b84e30acd515aadc4b783ad4ff83aff3299bdfe0";
				owner = "kr";
				repo = "logfmt";
				sha256 = "02ldzxgznrfdzvghfraslhgp19la1fczcbzh7wm2zdc6lmpd1qq9";
			};
			go-bindata-html-template = buildFromGitHub {
				rev = "4ae2add2b2f4e490b61ce4e7fdfa622824318375";
				owner = "arschles";
				repo = "go-bindata-html-template";
				sha256 = "0j5g3gpnrjn21m4wj9p63h79g15680v1cwpxjjsg77kapzsql5vv";
			};
			esc = buildFromGitHub {
				rev = "52c6e85770ad9de1c7168bb7e0725633320d114b";
				owner = "mjibson";
				repo = "esc";
				sha256 = "08zi5ig7m5hr8vgrv21vsc2bqdrxywbh4qs8q033zkqbj9ki4arp";
			};
			sqlx = buildFromGitHub {
				rev = "398dd5876282499cdfd4cb8ea0f31a672abe9495";
				owner = "jmoiron";
				repo = "sqlx";
				sha256 = "0533vj61kf56h5zwl95qx0cig01fkrbppphci82b5v7wikwqjpfc";
			};
			juju-errors = buildFromGitHub {
				rev = "b2c7a7da5b2995941048f60146e67702a292e468";
				owner = "juju";
				repo = "errors";
				sha256 = "1j2zwlibnqj21bws3bvzcqd0ss4dcvl84v9amadf1pzh3svz2hxi";
			};
			gopherjs = buildFromGitHub {
				rev = "d5d8eb5bbb7f4c0e10ccb62e1a45277cc823ff37";
				owner = "gopherjs";
				repo = "gopherjs";
				sha256 = "16gs13y9icbqyygxn7a5nff6fh1kigvw3qmp9bs7005mw89vqsmq";
				buildInputs = [
					astrewrite
					fsnotify
					tools
				] ++ (with pkgs.go16Packages; [
					osext
					sourcemap
					cobra
					pflag-spf13
					crypto
				]);
				patches = [./patches/gopherjs-gopath.patch];
			};
			astrewrite = buildFromGitHub {
				rev = "fad75ab64ef927530cab4b4dfbadaddc5f6b14e2";
				owner = "neelance";
				repo = "astrewrite";
				sha256 = "0jn87i51mwykw63x2lnv96p1zqq2yrhgalm79m73bicwggif3qnb";
			};
			sourcemap = buildFromGitHub {
				rev = "8c68805598ab8d5637b1a72b5f7d381ea0f39c31";
				owner = "neelance";
				repo = "sourcemap";
				sha256 = "0sg003hxmqhxrgkc2kyiph283s137z6f9javb5x8kcdg5pahmq7k";
			};
			fsnotify = buildFromGitHub {
				rev = "30411dbcefb7a1da7e84f75530ad3abe4011b4f8";
				owner = "fsnotify";
				repo = "fsnotify";
				sha256 = "0kbpvyi6p9942k0vmcw5z13mja47f7hq7nqd332pn2zydss6kddm";
				propagatedBuildInputs = [] ++ (with pkgs.go16Packages; [ sys ]);
			};
			tools = buildGoPackage {
				name = "go-tools-git";
				goPackagePath = "golang.org/x/tools";
				src = pkgs.fetchgit {
					url = "https://go.googlesource.com/tools/";
					rev = "2c200eec40d70d74e01852aad22cf6637e8f8f3d";
					sha256 = "0jqighkfxqv6rs4danhkfzc5411kk5lxfhcb3a3waj2w8qbi9xw7";
				};
				buildInputs = [] ++ (with pkgs.go16Packages; [net]);

				preConfigure = ''
					# Make the builtin tools available here
					mkdir -p $bin/bin
					eval $(go env | grep GOTOOLDIR)
					find $GOTOOLDIR -type f | while read x; do
						ln -sv "$x" "$bin/bin"
					done
					export GOTOOLDIR=$bin/bin
				'';

				excludedPackages = "\\("
					+ pkgs.stdenv.lib.concatStringsSep "\\|" ([ "testdata" ] ++ pkgs.stdenv.lib.optionals (pkgs.stdenv.lib.versionAtLeast pkgs.go.meta.branch "1.5") [ "vet" "cover" ])
					+ "\\)";

				# Do not copy this without a good reason for enabling
				# In this case tools is heavily coupled with go itself and embeds paths.
				allowGoReference = true;

				# Set GOTOOLDIR for derivations adding this to buildInputs
				postInstall = ''
					mkdir -p $bin/nix-support
					substituteAll ${<nixpkgs/pkgs/development/go-modules/tools/setup-hook.sh>} $bin/nix-support/setup-hook.tmp
					cat $bin/nix-support/setup-hook.tmp >> $bin/nix-support/setup-hook
					rm $bin/nix-support/setup-hook.tmp
				'';
		};
	}) pkgs;
	};

	focus = ((import ./default.nix) {
		inherit (pkgs) stdenv makeWrapper runCommand git;
		inherit goPackages;
	});
in
	focus