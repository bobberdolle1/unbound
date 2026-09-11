# Release lifecycle

## Pre-tag source gate

`wails.json` is the canonical source version. The frontend package metadata, engine fallback version, README, and CHANGELOG must match it. Local Go gates operate only on tracked Go files and package directories so generated dependencies, archived artifacts, and retained worktrees cannot affect a source verdict.

`Casks/unbound.rb` is intentionally excluded from this pre-tag gate. A Cask entry names a published macOS PKG and pins its actual SHA-256; it cannot be advanced before that PKG exists.

## Platform artifact sequence

1. Merge the verified source change and tag the exact merge commit.
2. Build and verify each platform from that exact tag.
3. Publish the immutable platform artifact.
4. For macOS, obtain the SHA-256 of the published PKG and submit a follow-up Cask metadata PR on `master`.

No placeholder Cask hash, copied prior-release artifact, or retroactive release-asset replacement is permitted.
