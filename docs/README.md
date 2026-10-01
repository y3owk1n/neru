# Neru documentation

Start with the guide if you use Neru, and with the contributing docs if you
want to change it. The reference pages are for looking up one exact option or
command.

## Guide

For people installing and using Neru.

- [Installation](guide/installation.md): every install method, including
  the one-line installer and Nix, and how to uninstall.
- [Getting started](guide/getting-started.md): first launch, permissions,
  your first config file, and your first hotkeys.
- [Recipes](guide/recipes.md): worked configurations for common workflows.
- [Troubleshooting](guide/troubleshooting.md): symptoms, causes and fixes.
- [Linux setup](guide/linux.md): libraries, permissions and the login service
  on Linux.
- [Linux desktops](guide/linux-desktops.md): notes for KDE Plasma, COSMIC,
  wlroots compositors, GNOME and X11.

## Reference

For looking things up.

- [Configuration](reference/configuration.md): every option in `config.toml`,
  with its default and the platforms it works on.
- [CLI](reference/cli.md): every `neru` command and flag.
- [Scripting and IPC](reference/scripting.md): driving Neru from scripts and
  other tools.
- [Platform support](reference/platform-support.md): what works on macOS,
  Linux and Windows.

## Project

- [Roadmap](project/roadmap.md): what comes next.
- [Showcases](project/showcases.md): configurations shared by the community.

## Contributing

For people changing Neru. Start with [CONTRIBUTING.md](../CONTRIBUTING.md).

- [Development](contributing/development.md): set up, build, test and debug.
- [Architecture](contributing/architecture.md): how Neru is put together and
  why.
- [Platform porting](contributing/porting.md): where platform code goes, and
  which doc owns which fact.
- [Objective-C style](contributing/objective-c.md): conventions for the macOS
  bridge.
- [Decision records](adr/): why the larger design choices went the way they
  did.
