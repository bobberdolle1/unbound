# Release lifecycle and immutability contract

## Immutable release sequence

1. Freeze the intended source commit. The working tree must be clean and the build identity must report the exact commit with `dirty=false`.
2. Create an **annotated** version tag on that commit. Never retarget, recreate, or force-move it.
3. Build every published platform artifact once from that tag. A release build injects `version`, `commit`, `dirty=false`, and `channel=release`; it never discovers Git metadata at runtime.
4. Generate one SHA-256 manifest from those artifacts before publication.
5. Create a draft GitHub Release, upload the artifacts and the manifest, then publish the completed draft. The workflow refuses an already-existing release for the same tag.
6. Download the published `SHA256SUMS.txt` and every published artifact; verify every remote digest against the locally generated manifest.
7. Run physical acceptance only with the published artifacts, then record an immutable acceptance manifest with the source commit, artifact digests, platform, date, and network context.

A released binary is never silently replaced under the same version. A binary defect requires a new source commit and a new version/tag.

GitHub immutable releases are available as a repository setting: **Settings → General → Releases → Enable release immutability**. Enable it before the next release. Immutable releases require the draft-upload-publish sequence because assets and metadata become locked at publication. The current v0.6.9 Release API reports `immutable=false`; policy and workflow safeguards apply until the GitHub setting is enabled.

## Build identity

`unbound --version` remains human-readable:

```text
unbound 0.7.0-dev (windows/amd64)
```

`unbound --version --json` emits:

```json
{
  "version": "0.7.0-dev",
  "commit": "0123456789abcdef0123456789abcdef01234567",
  "dirty": false,
  "channel": "development",
  "os": "windows",
  "arch": "amd64"
}
```

Direct ad-hoc builds use `commit="unknown"` and `dirty=null` when no build-time source metadata is supplied. Supported build entrypoints resolve that metadata before invoking Go/Wails. Release builds set `channel="release"`; development builds set `channel="development"`.

## v0.6.9 records

- Source identity and the current GitHub asset metadata: [`v0.6.9-identity-audit.md`](v0.6.9-identity-audit.md).
- Platform acceptance scope and constraints: [`../PLATFORMS.md`](../PLATFORMS.md).
- Linux v0.6.9 evidence: [`linux-v0.6.9-acceptance.md`](linux-v0.6.9-acceptance.md).
