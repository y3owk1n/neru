{
  autoPatchelfHook,
  fetchurl,
  gitUpdater,
  installShellFiles,
  stdenv,
  versionCheckHook,
  lib,
  buildGoModule,
  pkg-config,
  cairo,
  libei,
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
  unzip,
  apple-sdk_15 ? null,
}:
if useZip then
  let
    appName = "Neru.app";

    # Determine architecture-specific details
    archInfo =
      {
        "aarch64-darwin" = {
          url = "https://github.com/y3owk1n/neru/releases/download/v${version}/neru-darwin-arm64.zip";
          # run `nix hash convert --hash-algo sha256 (nix-prefetch-url https://github.com/y3owk1n/neru/releases/download/v1.50.0/neru-darwin-arm64.zip)`
          sha256 = "sha256-l1Tm+DxO5J+TN2B12iJo8Dw9fIYvNM0OITqTuCT9cTI=";
        };
        "x86_64-darwin" = {
          url = "https://github.com/y3owk1n/neru/releases/download/v${version}/neru-darwin-amd64.zip";
          # run `nix hash convert --hash-algo sha256 (nix-prefetch-url https://github.com/y3owk1n/neru/releases/download/v1.50.0/neru-darwin-amd64.zip)`
          sha256 = "sha256-s9UfMtjsShnM5W2V7tvg0PHpnZCBTtD9Wu56GRATRK8=";
        };
        "aarch64-linux" = {
          url = "https://github.com/y3owk1n/neru/releases/download/v${version}/neru-linux-arm64.zip";
          # run `nix hash convert --hash-algo sha256 (nix-prefetch-url https://github.com/y3owk1n/neru/releases/download/v1.50.0/neru-linux-arm64.zip)`
          sha256 = "sha256-LzOQekN5CVKE7sjJAjRNJqRSadxu2SJvJSAR5E7VrtU=";
        };
        "x86_64-linux" = {
          url = "https://github.com/y3owk1n/neru/releases/download/v${version}/neru-linux-amd64.zip";
          # run `nix hash convert --hash-algo sha256 (nix-prefetch-url https://github.com/y3owk1n/neru/releases/download/v1.50.0/neru-linux-amd64.zip)`
          sha256 = "sha256-oIVbbdw+Zxh5t8kUYtz8ieYktincq0+116ew7ztEYoM=";
        };
      }
      .${stdenv.hostPlatform.system} or (throw "Unsupported system: ${stdenv.hostPlatform.system}");

  in
  stdenv.mkDerivation {
    pname = "neru";

    inherit version;

    src = fetchurl {
      url = archInfo.url;
      sha256 = archInfo.sha256;
    };

    unpackPhase = ''
      unzip $src
    '';

    nativeBuildInputs = [
      installShellFiles
      unzip
    ]
    ++ lib.optionals stdenv.hostPlatform.isLinux [
      autoPatchelfHook
    ];

    buildInputs = lib.optionals stdenv.hostPlatform.isLinux [
      cairo
      libei
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

    installPhase = ''
      runHook preInstall
      ${
        if stdenv.hostPlatform.isDarwin then
          ''
            mkdir -p $out/Applications
            mv ${appName} $out/Applications
            cp -R bin $out
            mkdir -p $out/share/man/man1
            mv share/man/man1/*.1 $out/share/man/man1/
          ''
        else
          ''
            mkdir -p $out/bin
            mv bin/neru $out/bin/neru
            mkdir -p $out/share/man/man1
            mv share/man/man1/*.1 $out/share/man/man1/
          ''
      }
      runHook postInstall
    '';

    # only install completions on macOS
    # unable to make it work on Linux (do it manually please, sorry)
    postInstall = ''
      if ${
        lib.boolToString (
          stdenv.buildPlatform.canExecute stdenv.hostPlatform && stdenv.hostPlatform.isDarwin
        )
      }; then
        installShellCompletion --cmd neru \
              --bash <($out/Applications/Neru.app/Contents/MacOS/neru completion bash) \
              --fish <($out/Applications/Neru.app/Contents/MacOS/neru completion fish) \
              --zsh <($out/Applications/Neru.app/Contents/MacOS/neru completion zsh)
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

    src = lib.cleanSource ../.;

    # run the following command to get the sha256 hash
    # `nix-shell -p go --run 'go mod vendor'`
    # `nix hash path vendor`
    # `rm -rf vendor`
    vendorHash = "sha256-NU8b5M+KIrpODqci6QoQwgl28M1Eq3LiLBTnbZrhrmc=";

    ldflags = [
      "-s"
      "-w"
      "-X github.com/y3owk1n/neru/internal/buildinfo.Version=${finalAttrs.version}"
    ]
    ++ lib.optionals (commitHash != null) [
      "-X github.com/y3owk1n/neru/internal/buildinfo.GitCommit=${commitHash}"
    ];

    subPackages = [ "cmd/neru" ];

    nativeBuildInputs = [
      installShellFiles
      writableTmpDirAsHomeHook
    ]
    ++ lib.optionals stdenv.hostPlatform.isLinux [
      pkg-config
    ];

    buildInputs =
      lib.optionals stdenv.hostPlatform.isLinux [
        cairo
        libei
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
      ]
      ++ lib.optionals stdenv.hostPlatform.isDarwin [
        apple-sdk_15
      ];

    # Allow Go to use any available toolchain
    preBuild = ''
      export GOTOOLCHAIN=auto
    '';

    postInstall = ''
      # generate man pages
      mkdir -p $out/share/man/man1
      go run ./cmd/genman $out/share/man/man1

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
      cp ${finalAttrs.src}/resources/Neru.entitlements $out/Applications/Neru.app/Contents/Resources/Neru.entitlements

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
