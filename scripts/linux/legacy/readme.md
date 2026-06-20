# Legacy: standalone portable tar.gz builder (retired)

**Status: legacy / manual-only. Not built by the release flow.**

`build-release.sh` here is the previous standalone builder for the Linux portable
archive (`portier-<version>-linux.tar.gz`). The portable tar.gz is now produced by the
unified cross-platform generator (`scripts/build-portable.js`, via
`npm run build:release:current` / `build:release:portable:all`), so this script is kept
only as a reference.

The canonical Linux release scripts now live in the sibling
`scripts/linux/release/build-release.sh`, which builds the native **`.deb`**
(`portier_<version>_amd64.deb`) and **`.rpm`** (`portier-<version>-1.x86_64.rpm`) via
`--format deb|rpm`. That is the Linux counterpart to the macOS `build-release.sh` `.pkg` step
and the Windows WiX MSI.

These files are **not** part of `npm run build:release:current` and are not validated by
`npm run validate:release:*`. New Linux release/packaging work belongs in the parent
`scripts/linux/release/` (the `.deb`/`.rpm`) or `scripts/build-portable.js` (the tar.gz). See
`scripts/linux/readme.md` and `docs/installer.md`.

## Contents

- `build-release.sh` — previous standalone portable tar.gz builder.
