{
  autoPatchelfHook,
  fetchurl,
  gitUpdater,
  installShellFiles,
  makeWrapper,
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
  # tesseract backs the `vision` hint strategy on Linux. It is a required
  # dependency rather than an optional one: neru links libtesseract dynamically,
  # so a missing library stops the daemon before any neru code runs, whatever
  # hints.strategy is set to. The language data ships in the same derivation, at
  # $out/share/tessdata, and the wrapper below is what points neru at it — a Nix
  # store path is not somewhere the runtime search of /usr/share could find.
  tesseract,
  # pipewire is how KDE Plasma sessions read the screen. KWin implements no
  # screencopy protocol, so capture there goes through xdg-desktop-portal's
  # ScreenCast session and its frames arrive over PipeWire. Like tesseract it is
  # required rather than optional: neru links libpipewire-0.3 dynamically, so a
  # missing library stops the daemon before any neru code runs, whatever desktop
  # it was started on.
  pipewire,
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
          # run `nix hash convert --hash-algo sha256 (nix-prefetch-url https://github.com/y3owk1n/neru/releases/download/v1.51.0/neru-darwin-arm64.zip)`
          sha256 = "sha256-3kPIHczQLOlRRN8/skmKaVtM6SIyh63vaZPS7zFLFCA=";
        };
        "x86_64-darwin" = {
          url = "https://github.com/y3owk1n/neru/releases/download/v${version}/neru-darwin-amd64.zip";
          # run `nix hash convert --hash-algo sha256 (nix-prefetch-url https://github.com/y3owk1n/neru/releases/download/v1.51.0/neru-darwin-amd64.zip)`
          sha256 = "sha256-GXDU01tSx1oBUbzTPhSCkcHlIXwT46goo6i8iNxCtTw=";
        };
        "aarch64-linux" = {
          url = "https://github.com/y3owk1n/neru/releases/download/v${version}/neru-linux-arm64.zip";
          # run `nix hash convert --hash-algo sha256 (nix-prefetch-url https://github.com/y3owk1n/neru/releases/download/v1.51.0/neru-linux-arm64.zip)`
          sha256 = "sha256-wYGt20QCfQ5sQ8Le0jxfAa1jzeHdhvm/mTicdPKxbCA=";
        };
        "x86_64-linux" = {
          url = "https://github.com/y3owk1n/neru/releases/download/v${version}/neru-linux-amd64.zip";
          # run `nix hash convert --hash-algo sha256 (nix-prefetch-url https://github.com/y3owk1n/neru/releases/download/v1.51.0/neru-linux-amd64.zip)`
          sha256 = "sha256-h8d/FzWIcczJnn2mflfdjVpFi4tfO35d8QCF2z8Zp9Q=";
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
      makeWrapper
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
      tesseract
      pipewire
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
    ''
    + lib.optionalString stdenv.hostPlatform.isLinux ''
      # --set-default, so a user who exports their own TESSDATA_PREFIX (a
      # tessdata_fast checkout, another language) keeps it.
      wrapProgram $out/bin/neru \
        --set-default TESSDATA_PREFIX ${tesseract}/share/tessdata
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
    vendorHash = "sha256-EkmJ2Pr2PjbiKNZpLfpzVXn4NDmBAKO6rxirIQOL7tQ=";

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
      makeWrapper
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
        tesseract
        pipewire
      ]
      ++ lib.optionals stdenv.hostPlatform.isDarwin [
        apple-sdk_15
      ];

    # pipewire's libspa-0.2.pc carries `-fno-strict-overflow` in its Cflags, and
    # libpipewire-0.3 pulls it in via Requires. cgo refuses any pkg-config flag
    # outside its allowlist, and that one is not on it, so the linux adapter
    # fails to build until cgo is told the flag is safe.
    env = lib.optionalAttrs stdenv.hostPlatform.isLinux {
      CGO_CFLAGS_ALLOW = "-fno-strict-overflow";
    };

    # Build with the toolchain nixpkgs ships, never a downloaded one. go.mod
    # carries a `toolchain` line pointing at a Go patch release newer than
    # nixpkgs has, so that everyone building with a network — contributors, CI,
    # the release workflow — picks up the fixed standard library. This sandbox
    # has no network, so honouring that line would fail the build outright
    # rather than silently fall back. `local` ignores it and uses the nixpkgs
    # Go, which still satisfies the `go` directive (the actual floor). Nix
    # builds therefore track nixpkgs' Go patch level, as they always have, and
    # pick the fixed standard library up when nixpkgs does.
    preBuild = ''
      export GOTOOLCHAIN=local
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
    + lib.optionalString stdenv.hostPlatform.isLinux ''
      # tesseract resolves its language data at run time, and it lives in a
      # store path no filesystem search would find. --set-default, so a user who
      # exports their own TESSDATA_PREFIX keeps it.
      wrapProgram $out/bin/neru \
        --set-default TESSDATA_PREFIX ${tesseract}/share/tessdata
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
