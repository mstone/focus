{ pkgs ? import <nixpkgs> {} }:
let
	focus = ((import ./shell.nix) { inherit pkgs; }).bin;
in pkgs.dockerTools.buildImage {
	name = "focus";
	tag = "latest";

	fromImage = pkgs.dockerTools.buildImage {
		name = "bash";
		tag = "latest";
		contents = pkgs.bash;
	};

	contents = focus;

	runAsRoot = ''
		mkdir -p /data
	'';

	config = {
		Cmd = [ "/bin/focus" ];
		WorkingDir = "/data";
		Volumes = {
			"/data" = {};
		};
	};
}