# Neru documentation

New to Neru? Read **Start here** in order. The guides solve one task each,
concepts explain how Neru thinks, and the reference lists every option and
command for lookup.

## Start here

- [Installation](guide/installation.md): install, update and remove Neru.
- [Getting started](guide/getting-started.md): start the daemon, grant
  permissions and bind your first hotkey.
- [Using Neru](guide/using-neru.md): a ten-minute tour of every mode.

## Guides

- [Configuring Neru](guide/configuring.md): where the config lives and how to
  apply changes.
- [Recipes](guide/recipes.md): ready-to-copy configs for common goals.
- [Scripting](guide/scripting.md): drive Neru from scripts and other hotkey
  tools.

## Linux

- [Linux setup](guide/linux.md): libraries, permissions and the login service.
- [Linux desktops](guide/linux-desktops.md): notes for KDE Plasma, COSMIC,
  wlroots compositors, GNOME and X11.

## Concepts

- [How bindings work](concepts/bindings.md): what a binding can run, and which
  one answers a key.
- [Glossary](concepts/glossary.md): the words these docs use.

## Reference

- [Configuration](reference/configuration.md): every option in `config.toml`.
- [CLI](reference/cli.md): every `neru` command and flag.
- [IPC protocol](reference/ipc.md): the JSON format the CLI and daemon speak.
- [Platform support](reference/platform-support.md): what works on macOS,
  Linux and Windows.

## Help

- [Troubleshooting](guide/troubleshooting.md): symptoms, causes and fixes.

## Project

- [Roadmap](project/roadmap.md): what comes next.
- [Community configurations](project/showcases.md): setups shared by users.
- [Changelog](../CHANGELOG.md): what changed in each release.

## Contributing

These stay on GitHub. Start with [CONTRIBUTING.md](../CONTRIBUTING.md).

- [Development](contributing/development.md): set up, build, test and debug.
- [Architecture](contributing/architecture.md): how Neru is put together.
- [Platform porting](contributing/porting.md): where platform code goes, and
  which doc owns which fact.
- [Platform internals](contributing/platform-internals.md): how each platform
  implements each capability.
- [Objective-C style](contributing/objective-c.md): conventions for the macOS
  bridge.
- [Decision records](adr/): why the larger design choices went the way they
  did.
