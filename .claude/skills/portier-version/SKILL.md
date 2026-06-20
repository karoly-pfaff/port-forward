# Portier Version Bump

Use this skill to change or verify the Portier release version. There is **one source of
truth** (the root `package.json` `version`) and one script that propagates it to every other
surface. Never hand-edit a version string in an individual file — that is exactly how the client
sidebar shipped `v1.14.1` across four releases.

## When To Use

- Bumping the version for a release (`1.18.0 → 1.19.0`).
- Verifying every version surface is consistent (before tagging, in review, after a merge).
- Any time you are tempted to edit a `"version"`, `PORTIER_APP_VERSION`, `var Version`, or
  `OPENAPI_DOC_VERSION` by hand.

## Commands

```bash
npm run version:list            # show every surface's current value
npm run version:check           # verify all surfaces match root; non-zero exit on drift
npm run version:set 1.19.0      # write the version to every surface (semver required)
npm run version:bump minor      # compute the next major|minor|patch from root, then set
```

`version:check` runs in CI (the **Scripts** + **Sanity** jobs) and in `npm run check`, so drift
fails the build instead of shipping.

## Surfaces (managed automatically — do not edit by hand)

Full version (e.g. `1.19.0`): root/`client`/`server`/`shared` `package.json`,
`shared/sources/index.ts` `PORTIER_APP_VERSION`, and the three Go `version.go` files
(`service`, `tools/cli`, `tools/replay`).

OpenAPI major.minor (e.g. `1.19`): `server/sources/openapi/openapi.ts` `OPENAPI_DOC_VERSION`
and `docs/openapi.json` `info.version`.

The single list lives in `scripts/generate-version.js` (`SURFACES`). To bring a **new** surface under the
tool, add one entry there — that is the only change required.

## Workflow

1. `npm run version:set <x.y.z>` (or `version:bump <level>`).
2. If the API itself also changed, regenerate the OpenAPI doc: `npm run apidoc:generate`
   (`version:set` already patched `docs/openapi.json` `info.version` in place, so this is only
   needed for real schema changes).
3. `npm run version:check` to confirm all surfaces agree.
4. Build/validate as the change warrants, then commit. Tagging/releasing stays a maintainer step.

## Guardrails

- `package-lock.json`'s top-level `version` is intentionally **not** a maintained surface (stale
  by history); leave it.
- The version is a sync target, not a feature: bumping it is a release action. Do not bump during
  RC-stabilization slices unless the task is the release itself.
