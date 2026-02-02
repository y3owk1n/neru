{
  fetchzip,
  gitUpdater,
  installShellFiles,
  stdenv,
  versionCheckHook,
  lib,
  buildGoModule,
  version ? "main",
  useZip ? false,
  commitHash ? null,
  writableTmpDirAsHomeHook,
  nix-update-script,
}:
if useZip then
  let
    appName = "Neru.app";

    # Determine architecture-specific details
    archInfo =
      {
        "aarch64-darwin" = {
          url = "https://github.com/y3owk1n/neru/releases/download/v${version}/neru-darwin-arm64.zip";
          # run `nix hash convert --hash-algo sha256 (nix-prefetch-url --unpack https://github.com/y3owk1n/neru/releases/download/v1.17.0/neru-darwin-arm64.zip)`
          sha256 = "sha256-GU92lkpbE8YBaIcxHnPSs7mblmAx886sqgVRvWNdIU8=";
        };
        "x86_64-darwin" = {
          url = "https://github.com/y3owk1n/neru/releases/download/v${version}/neru-darwin-amd64.zip";
          # run `nix hash convert --hash-algo sha256 (nix-prefetch-url --unpack https://github.com/y3owk1n/neru/releases/download/v1.17.0/neru-darwin-amd64.zip)`
          sha256 = "sha256-Qh/oP5XJ7oeUafz5J/FHCaajdrsahglP5bHjlaGuf/k=";
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
      if ${lib.boolToString (stdenv.buildPlatform.canExecute stdenv.hostPlatform)}; then
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

    meta = with lib; {
      description = "Navigate macOS without touching your mouse";
      homepage = "https://github.com/y3owk1n/neru";
      license = licenses.mit;
      platforms = platforms.darwin;
      mainProgram = "neru";
    };
  }
else
  let
    shortHash = if commitHash != null then lib.substring 0 7 commitHash else null;

    pversion = "${version}${if shortHash != null then "-${shortHash}" else ""}";
  in
  # Build from source
  buildGoModule (finalAttrs: {
    pname = "neru";
    version = pversion;

    src = lib.cleanSource ./.;

    # run the following command to get the sha256 hash
    # `nix-shell -p go --run 'go mod vendor'`
    # `nix hash path vendor`
    # `rm -rf vendor`
    vendorHash = "sha256-1ECRW+Rq5wXXk+TDLMlBo2TcWkg3HbY8mPFWtRh/E+s=";

    ldflags = [
      "-s"
      "-w"
      "-X github.com/y3owk1n/neru/internal/cli.Version=${finalAttrs.version}"
    ]
    ++ lib.optionals (commitHash != null) [
      "-X github.com/y3owk1n/neru/internal/cli.GitCommit=${commitHash}"
    ];

    # Completions
    nativeBuildInputs = [
      installShellFiles
      writableTmpDirAsHomeHook
    ];

    subPackages = [ "cmd/neru" ];

    # Allow Go to use any available toolchain
    preBuild = ''
      export GOTOOLCHAIN=auto
    '';

    postInstall = ''
      # install shell completions
      if ${lib.boolToString (stdenv.buildPlatform.canExecute stdenv.hostPlatform)}; then
      	installShellCompletion --cmd neru \
      	--bash <($out/bin/neru completion bash) \
      	--fish <($out/bin/neru completion fish) \
      	--zsh <($out/bin/neru completion zsh)
      fi

      # Create a simple .app bundle on the fly
      mkdir -p $out/Applications/Neru.app/Contents/{MacOS,Resources}

      cp $out/bin/neru $out/Applications/Neru.app/Contents/MacOS/Neru

      SRC_PLIST=${finalAttrs.src}/resources/Info.plist.template

      sed "s|VERSION|${finalAttrs.version}|g" $SRC_PLIST > $out/Applications/Neru.app/Contents/Info.plist

      echo "✅ Neru.app bundle created at $out/Applications/Neru.app"
    '';

    passthru = {
      updateScript = nix-update-script { };
    };

    meta = with lib; {
      description = "Navigate macOS without touching your mouse";
      homepage = "https://github.com/y3owk1n/neru";
      license = licenses.mit;
      platforms = platforms.darwin;
      mainProgram = "neru";
    };
  })
