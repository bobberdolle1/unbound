# Provider-independent CI

Repository scripts are the CI contract. Buildkite and GitHub Actions only provision a checkout and toolchains, then invoke the platform entrypoint.

| Platform | Command |
| --- | --- |
| Windows | `pwsh -File scripts/ci/windows.ps1` |
| Linux | `bash scripts/ci/linux.sh` |
| macOS | `bash scripts/ci/macos.sh` |
| Frontend only | `bash scripts/ci/frontend.sh` |

Each entrypoint prints `EXPECTED_SHA` and `ACTUAL_SHA` and fails unless they match. In Buildkite, `BUILDKITE_COMMIT` is the expected SHA; in GitHub Actions, `GITHUB_SHA` is used. Manual runs use the checked-out `HEAD`.

## Contract

- Go must be compatible with the `go` directive in `go.mod` (currently Go 1.26.0).
- Buildkite agents must provision Node 22.13.0, matching repository CI configuration. The scripts reject Node versions older than that major version and print the actual version.
- Linux owns workflow semantics (`actionlint`), asset hashing, shell syntax, cross-platform engine builds, frontend checks, and Go checks.
- Windows additionally runs the real `winws2 --dry-run` parser suite. It does not run WinDivert, packet interception, or privileged acceptance.
- macOS runs ordinary Go tests and a native core build. It does not run `tpws` measurement acceptance.
- The Linux validation Dockerfile is optional and must be invoked explicitly; ordinary CI does not require Docker or root.

## Buildkite V1 trust boundary

Buildkite is the primary orchestrator. Its agents are personal, non-disposable machines. Configure the Buildkite GitHub integration to accept only canonical-repository pushes and manually approved canonical-repository PR builds. Disable fork-PR automatic builds before registering an agent. Do not give an agent token, checkout key, or agent user administrator/root privileges.

Run agents under dedicated unprivileged users with an isolated Buildkite build path. Configure agent cleanup outside this repository so each job removes its workspace and cannot inspect unrelated home directories. Never configure `sudo`, firewall changes, DNS changes, routing changes, proxies, NFQUEUE, WinDivert, or persistent packet interception in a CI job.

No Buildkite token, SSH key, machine address, or credential belongs in this repository. Agent provisioning is an account-administration action, not a repository script.

## Provider roles

- **GitHub**: canonical source repository.
- **Repository scripts**: canonical build and test behavior.
- **Buildkite**: primary CI orchestrator once agents are registered.
- **GitHub Actions**: secondary consumer of the same scripts; it may remain unavailable while the account billing lock exists.
