# Firmware Updates for Prusa Connect

A [Prusa Connect](https://github.com/prusa3d/prusa-connect) extension that tells you when new
firmware is out for your Prusa printers, and installs it only when you ask.

- Reads the releases of [Prusa-Firmware-Buddy](https://github.com/prusa3d/Prusa-Firmware-Buddy/releases)
  every few hours.
- Finds the right image for every printer: CORE One, CORE One L, their INDX variants, MK4, MK4S,
  MK3.9, MK3.9S, MK3.5, MK3.5S, XL and MINI (in the language you choose).
- Each printer follows the **stable** channel, or the **beta** channel if you choose it for that
  printer. Beta offers alpha, beta and release-candidate firmware.
- **Nothing is installed by itself.** Updating takes two steps, each confirmed: copy the image to
  the printer's USB drive, then start the installation.

## Where it shows

| place | what |
| --- | --- |
| printer listing | one line on each printer's card when an update is available |
| printer → Status tab | installed and newest version, channel (Default / Stable / Beta), release notes, *Copy to the printer* and *Install now* |
| sidebar → Firmware Updates | every printer with its channel and newest version, and *Check now* |
| Settings → Extensions → Firmware Updates | the settings below |

## Settings

| key | default | what |
| --- | --- | --- |
| `default_channel` | `stable` | the channel of a printer nobody chose one for: `stable` or `beta` |
| `mini_language` | `german` | every MINI image holds English and one more language; this picks it |
| `check_hours` | `6` | how often GitHub is read |
| `storage` | `local` | the storage of this server an image is downloaded into before it goes to the printer |
| `github_token` | none | optional, only when 60 GitHub requests an hour from your server are not enough |

## How a file is checked

Before anything reaches a printer, the downloaded image must

1. be listed in the release for the printer's model, by its file name;
2. match the size and the SHA-256 GitHub publishes for it (where GitHub publishes one);
3. carry, in its own header, the printer code of that model and the release's version.

A file that fails any check is refused and never sent. The image is copied to the printer's USB
drive under an 8.3 name such as `FW802.BBF`, and the flash is the printer's own `M997`, sent only
when you press *Install now* and confirm. Install only while the printer is not printing, and do
not switch it off while it flashes.

## What it asks for

| request | why |
| --- | --- |
| `write_storage` | the image is written into a storage of your server, so it can be sent to the printer from there |
| a data directory | the release list and the channel chosen for each printer |
| `api.github.com`, `github.com`, `*.githubusercontent.com` | the release list and the images; nothing else is sent |

Copying to the printer and starting the flash run as the person who pressed the button, so they
need *operate* on that printer.

## Build

Needs Go (the version in `go.mod`), Node 22.12+ and pnpm (`corepack enable`).

```bash
make check     # Go vet and tests, the client is current, the UI's lint, types and build
make release   # release/: module.yaml, ui/, one binary per platform in module.yaml
make install   # installs release/ into bin/try with Prusa Connect's own `module add`
```

Until the Prusa Connect SDK and the `@prusa` npm packages are published, both halves build
against a checkout of Prusa Connect at `CORE_REF` (Makefile), which `make` clones into `.core/`.

## Publish

1. Raise `version` in `module.yaml` and add its heading to `CHANGELOG.md`.
2. Publish a GitHub release tagged `v<version>`. A pre-release goes to the marketplace's beta
   channel. `release.yml` hands the commit to the marketplace's builder.

## Disclaimer

This is not an official Prusa Research product. Firmware is downloaded from Prusa's public
GitHub releases and installed only on your confirmation; you install it at your own risk.

## License

MIT, see `LICENSE`.
