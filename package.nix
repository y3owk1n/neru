{
  fetchzip,
  gitUpdater,
  installShellFiles,
  stdenv,
  versionCheckHook,
  lib,
  buildGoModule,
  pkg-config,
  cairo,
  libxkbcommon,
  wayland,
  wayland-protocols,
  libx11,
  libxext,
  libxfixes,
  libxrandr,
  libxrender,
  libxtst,
  libxi,
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
          # run `nix hash convert --hash-algo sha256 (nix-prefetch-url --unpack https://github.com/y3owk1n/neru/releases/download/v1.32.0/neru-darwin-arm64.zip)`
          sha256 = "sha256-RxLeOSn4iXJmPyiQMe6uykdc2VSFCwN3llNY+aN/0vM=";
        };
        "x86_64-darwin" = {
          url = "https://github.com/y3owk1n/neru/releases/download/v${version}/neru-darwin-amd64.zip";
          # run `nix hash convert --hash-algo sha256 (nix-prefetch-url --unpack https://github.com/y3owk1n/neru/releases/download/v1.32.0/neru-darwin-amd64.zip)`
          sha256 = "sha256-25VUxQGq2K+nbTjEGipGHcy4i6OUw1BLsUHkoIX0gE4=";
        };
        "aarch64-linux" = {
          url = "https://github.com/y3owk1n/neru/releases/download/v${version}/neru-linux-arm64.zip";
          # run `nix hash convert --hash-algo sha256 (nix-prefetch-url --unpack https://github.com/y3owk1n/neru/releases/download/v1.32.0/neru-linux-arm64.zip)`
          sha256 = "sha256-/KlBmHiviTqEMrs8VsRfvdGVkhc5yr62BPL0Z0H7dZw=";
        };
        "x86_64-linux" = {
          url = "https://github.com/y3owk1n/neru/releases/download/v${version}/neru-linux-amd64.zip";
          # run `nix hash convert --hash-algo sha256 (nix-prefetch-url --unpack https://github.com/y3owk1n/neru/releases/download/v1.32.0/neru-linux-amd64.zip)`
          sha256 = "sha256-YjG2/jrTz8QVgBKeP8cQWQkFH7B9wqUY8d2vtaHnDuU=";
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
      ${
        if stdenv.hostPlatform.isDarwin then
          ''
            mkdir -p $out/Applications
            mv ${appName} $out/Applications
            cp -R bin $out
            mkdir -p $out/share
            runHook postInstall
          ''
        else
          ''
            mkdir -p $out/bin
            mv bin/neru $out/bin/neru
          ''
      }
      runHook postInstall
    '';

    postInstall = ''
      if ${lib.boolToString (stdenv.buildPlatform.canExecute stdenv.hostPlatform)}; then
        installShellCompletion --cmd neru \
        ${
          if stdenv.hostPlatform.isDarwin then
            ''
              --bash <($out/Applications/Neru.app/Contents/MacOS/neru completion bash) \
              --fish <($out/Applications/Neru.app/Contents/MacOS/neru completion fish) \
              --zsh <($out/Applications/Neru.app/Contents/MacOS/neru completion zsh)
            ''
          else
            ''
              --bash <($out/bin/neru completion bash) \
              --fish <($out/bin/neru completion fish) \
              --zsh <($out/bin/neru completion zsh)
            ''
        }
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
      platforms = platforms.darwin ++ platforms.linux;
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
    vendorHash = "sha256-VQFqWNlZV7ap2zNvkqjzmfjkqBTejMX3pqyEFAAVQVI=";

    ldflags = [
      "-s"
      "-w"
      "-X github.com/y3owk1n/neru/internal/cli.Version=${finalAttrs.version}"
    ]
    ++ lib.optionals (commitHash != null) [
      "-X github.com/y3owk1n/neru/internal/cli.GitCommit=${commitHash}"
    ];

    nativeBuildInputs = [
      installShellFiles
      writableTmpDirAsHomeHook
    ]
    ++ lib.optionals stdenv.hostPlatform.isLinux [
      pkg-config
    ];

    buildInputs = lib.optionals stdenv.hostPlatform.isLinux [
      cairo
      libxkbcommon
      wayland
      wayland-protocols
      libx11
      libxext
      libxfixes
      libxrandr
      libxrender
      libxtst
      libxi
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
    ''
    + lib.optionalString stdenv.hostPlatform.isDarwin ''
      # Create a simple .app bundle on the fly for macOS source builds.
      mkdir -p $out/Applications/Neru.app/Contents/{MacOS,Resources}

      cp $out/bin/neru $out/Applications/Neru.app/Contents/MacOS/neru

      cp ${finalAttrs.src}/resources/icon.icns $out/Applications/Neru.app/Contents/Resources/icon.icns

      SRC_PLIST=${finalAttrs.src}/resources/Info.plist.template

      sed "s|VERSION|${finalAttrs.version}|g" $SRC_PLIST > $out/Applications/Neru.app/Contents/Info.plist

      echo "✅ Neru.app bundle created at $out/Applications/Neru.app"
    '';

    passthru = {
      updateScript = nix-update-script { };
    };

    meta = with lib; {
      description = "Keyboard-driven navigation tool for macOS and Linux";
      homepage = "https://github.com/y3owk1n/neru";
      license = licenses.mit;
      platforms = platforms.darwin ++ platforms.linux;
      mainProgram = "neru";
    };
  })
