{ dockerTools, focus, bash }:
dockerTools.buildImage {
	name = "focus";
	tag = "latest";

	fromImage = dockerTools.buildImage {
		name = "bash";
		tag = "latest";
		contents = bash;
	};

	contents = focus.bin;

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
