#!/usr/bin/env bash
# Verify a staged local release manifest and its detached SHA-256 manifest.
set -euo pipefail

if [[ $# -ne 3 ]]; then
    printf 'usage: %s VERSION SOURCE_COMMIT RELEASE_DIRECTORY\n' "$0" >&2
    exit 2
fi

VERSION="$1"
COMMIT="$2"
DIRECTORY="$3"
MANIFEST="$DIRECTORY/RELEASE_MANIFEST.json"
SUMS="$DIRECTORY/SHA256SUMS.txt"
[[ -s "$MANIFEST" && -s "$SUMS" ]] || { printf 'release manifest or SHA256SUMS is missing\n' >&2; exit 1; }
(
    cd "$DIRECTORY"
    if command -v shasum >/dev/null 2>&1; then
        shasum -a 256 -c SHA256SUMS.txt
    else
        sha256sum -c SHA256SUMS.txt
    fi
)
VERSION="$VERSION" COMMIT="$COMMIT" MANIFEST="$MANIFEST" node - <<'NODE'
const fs = require('fs');
const manifest = JSON.parse(fs.readFileSync(process.env.MANIFEST, 'utf8'));
if (manifest.version !== process.env.VERSION || manifest.source_commit !== process.env.COMMIT || manifest.tag !== `v${process.env.VERSION}` || manifest.build_channel !== 'release' || !Array.isArray(manifest.artifacts) || manifest.artifacts.length !== 4 || !manifest.artifacts.every(artifact => artifact.name && artifact.sha256 && artifact.platform && artifact.architecture && artifact.build_host_role && artifact.built_at)) process.exit(1);
NODE
printf 'LOCAL_RELEASE_ARTIFACT_VERIFICATION=PASS directory=%s\n' "$DIRECTORY"
