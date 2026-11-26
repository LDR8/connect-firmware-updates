# Changelog

What an operator sees change when they upgrade this module, newest release
first. `version:` in `module.yaml` is the newest heading below.

## Unreleased

### Fixed

- A printer whose model is spelled in another case than expected, such as a CORE One reporting
  `COREONE`, can now copy its update to the printer. Before, the update was offered, but the
  image check refused it as an unknown model.

## 0.1.0 - 2026-11-18

### Added

- Reads the releases of [Prusa-Firmware-Buddy](https://github.com/prusa3d/Prusa-Firmware-Buddy/releases)
  every `check_hours` hours and finds the newest firmware for every printer model.
- Each printer follows the stable channel or, when chosen on its Status tab, the beta channel
  (alpha, beta and release-candidate firmware). The setting `default_channel` sets the channel of
  a printer nobody chose one for.
- A line on every printer card when an update is available, a section on the printer's Status tab,
  and a Firmware Updates page listing every printer.
- Installs only on request: "Copy to the printer" downloads the image and puts it on the
  printer's USB drive, and "Install now" starts the flash. Both ask for confirmation.
- Every downloaded image is held to the size and SHA-256 GitHub publishes for it, and to the
  printer code and version in its own header. A file that does not fit the printer is never sent.
