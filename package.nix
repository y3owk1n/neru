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
  darwin,
}:
if useZip then
  let
    appName = "Neru.app";

    # Determine architecture-specific details
    archInfo =
      {
        "aarch64-darwin" = {
          url = "https://github.com/y3owk1n/neru/releases/download/v${version}/neru-darwin-arm64.zip";
          # run `nix hash convert --hash-algo sha256 (nix-prefetch-url --unpack https://github.com/y3owk1n/neru/releases/download/v1.14.0/neru-darwin-arm64.zip)`
          sha256 = "sha256-ej1beRtD/CQGFhRyvQNZwR6VzTFZho57uRbZn07SaTc=";
        };
        "x86_64-darwin" = {
          url = "https://github.com/y3owk1n/neru/releases/download/v${version}/neru-darwin-amd64.zip";
          # run `nix hash convert --hash-algo sha256 (nix-prefetch-url --unpack https://github.com/y3owk1n/neru/releases/download/v1.14.0/neru-darwin-amd64.zip)`
          sha256 = "sha256-brHb/7SNmcgEsg/uvmNMUReYq4A5rcI2INHfuYXq/6U=";
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
    vendorHash = "sha256-PjLfHZSHB1LV6mhbI0ER+jU9+wSOVcSREsfyYfzbrpM=";

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
      darwin.sigtool # Provides codesign for ad-hoc signing
    ];

    subPackages = [ "cmd/neru" ];

    # Allow Go to use any available toolchain
    preBuild = ''
      export GOTOOLCHAIN=auto
    '';

    postInstall = ''
      	# install shell completions
      	if ${lib.optionalString (stdenv.buildPlatform.canExecute stdenv.hostPlatform) "true"}; then
      	installShellCompletion --cmd neru \
      		--bash <($out/bin/neru completion bash) \
      		--fish <($out/bin/neru completion fish) \
      		--zsh <($out/bin/neru completion zsh)
      	fi

      	# Create a simple .app bundle on the fly
      	mkdir -p $out/Applications/Neru.app/Contents/{MacOS,Resources}

      	cp $out/bin/neru $out/Applications/Neru.app/Contents/MacOS/Neru

       	sed "s/VERSION/${finalAttrs.version}/g" resources/Info.plist.template > $out/Applications/Neru.app/Contents/Info.plist

      	# Sign the entire app bundle
      	echo "🔐 Code signing app bundle..."
      	codesign --force --deep --sign - $out/Applications/Neru.app

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
