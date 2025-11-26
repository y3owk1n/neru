{
  fetchzip,
  gitUpdater,
  installShellFiles,
  stdenv,
  versionCheckHook,
  lib,
}:
let
  appName = "Neru.app";
  version = "1.10.2";

  # Determine architecture-specific details
  archInfo =
    {
      "aarch64-darwin" = {
        url = "https://github.com/y3owk1n/neru/releases/download/v${version}/neru-darwin-arm64.zip";
        # run `nix hash convert --hash-algo sha256 (nix-prefetch-url --unpack https://github.com/y3owk1n/neru/releases/download/v1.10.2/neru-darwin-arm64.zip)`
        sha256 = "sha256-TjIcevj7VKkrrvu6WiJxXKwqDuKsWc55269sobIGA3s=";
      };
      "x86_64-darwin" = {
        url = "https://github.com/y3owk1n/neru/releases/download/v${version}/neru-darwin-amd64.zip";
        # run `nix hash convert --hash-algo sha256 (nix-prefetch-url --unpack https://github.com/y3owk1n/neru/releases/download/v1.10.2/neru-darwin-amd64.zip)`
        sha256 = "sha256-FFooL65CU0sr4ZICXxPcZf8MQNBgZn91wAI4o9Gh/vo=";
      };
    }
    .${stdenv.hostPlatform.system} or (throw "Unsupported system: ${stdenv.hostPlatform.system}");

in
stdenv.mkDerivation {
  pname = "neru";
  inherit version;
  src = fetchzip {
    url = archInfo.url;
    sha256 = archInfo.sha256;
    stripRoot = false;
  };
  nativeBuildInputs = [ installShellFiles ];
  installPhase = ''
    runHook preInstall
    mkdir -p $out/Applications
    mv ${appName} $out/Applications
    cp -R bin $out
    mkdir -p $out/share
    runHook postInstall
  '';
  postInstall = ''
    if ${lib.optionalString (stdenv.buildPlatform.canExecute stdenv.hostPlatform) "true"}; then
      installShellCompletion --cmd neru \
        --bash <($out/bin/neru completion bash) \
        --fish <($out/bin/neru completion fish) \
        --zsh <($out/bin/neru completion zsh)
    fi
  '';
  doInstallCheck = true;
  nativeInstallCheckInputs = [
    versionCheckHook
  ];
  passthru.updateScript = gitUpdater {
    url = "https://github.com/y3owk1n/neru.git";
    rev-prefix = "v";
  };
  meta = {
    mainProgram = "neru";
    platforms = [
      "aarch64-darwin"
      "x86_64-darwin"
    ];
  };
}
