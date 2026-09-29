# Release lifecycle and immutability contract

## Immutable release sequence

1. Freeze the intended source commit. The working tree must be clean and the build identity must report the exact commit with `dirty=false`.
2. Create an **annotated** version tag on that commit. Never retarget, recreate, or force-move it.
3. Build every published platform artifact once from that tag. A release build injects `version`, `commit`, `dirty=false`, and `channel=release`; it never discovers Git metadata at runtime.
4. Generate one SHA-256 manifest from those artifacts before publication.
5. Create a draft GitHub Release, upload the artifacts and manifest, then download and verify every **draft** asset against the local manifest. The workflow refuses any existing release for the tag.
6. Publish only after draft verification passes; download the published manifest and artifacts again to verify their digests.
7. Run physical acceptance only with the published artifacts, then record an immutable acceptance manifest with the source commit, artifact digests, platform, date, and network context.

A released binary is never silently replaced under the same version. A binary defect requires a new source commit and a new version/tag.

GitHub release immutability is not currently enforced by repository settings, so policy safeguards apply: annotated tag, no retarget, draft upload, remote download verification, and a single publication. GitHub Actions is unavailable on the linked account and is never a release authority; local canonical gates are authoritative.

## Local release gates

Локальный coordinator запускает `scripts/release/local_release_windows.ps1 VERSION EXPECTED_COMMIT` на Windows release-host, `scripts/release/local_release_macos.sh VERSION EXPECTED_COMMIT` на macOS release-host и `scripts/release/local_release_linux.sh VERSION EXPECTED_COMMIT` на Linux `x86_64` release-host. Каждый отклоняет dirty/wrong checkout, запускает repository-owned canonical gate, внедряет release identity, собирает только поддерживаемый артефакт, проверяет embedded identity и пишет machine-readable evidence. Linux coordinator создаёт только экспериментальный CLI `tar.gz`; он не включает GUI.

Для сборки единого `SHA256SUMS.txt` и `RELEASE_MANIFEST.json` используйте `scripts/release/assemble_local_release.sh VERSION EXPECTED_COMMIT OUTPUT_DIRECTORY ARTIFACT...`; ему нужны фактические `UNBOUND_WINDOWS_BUILT_AT`, `UNBOUND_MACOS_BUILT_AT` и `UNBOUND_LINUX_BUILT_AT` из platform evidence. Linux entry имеет `platform=linux`, `architecture=amd64`, `experimental=true`. До и после upload запускайте `scripts/release/verify_release_artifacts.sh VERSION EXPECTED_COMMIT OUTPUT_DIRECTORY`. Эти scripts не используют GitHub Actions как release authority.


## Build identity

`unbound --version` remains human-readable:

```text
unbound 0.7.0 (windows/amd64)
```

`unbound --version --json` for a release artifact emits:

```json
{
  "version": "0.7.0",
  "commit": "0123456789abcdef0123456789abcdef01234567",
  "dirty": false,
  "channel": "release",
  "os": "windows",
  "arch": "amd64"
}
```

Direct ad-hoc builds use `commit="unknown"` and `dirty=null` when no build-time source metadata is supplied. Supported build entrypoints resolve that metadata before invoking Go/Wails. Release builds set `channel="release"`; development builds set `channel="development"`.

## v0.6.9 records

- Source identity and the current GitHub asset metadata: [`v0.6.9-identity-audit.md`](v0.6.9-identity-audit.md).
- Platform acceptance scope and constraints: [`../PLATFORMS.md`](../PLATFORMS.md).
- Linux v0.6.9 evidence: [`linux-v0.6.9-acceptance.md`](linux-v0.6.9-acceptance.md).
