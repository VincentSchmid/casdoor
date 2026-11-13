# Casdoor with PR #6079

Build the latest stable Casdoor release with the fix for shared CIMD clients
using the built-in signing certificate instead of their organization's certificate.

- Issue: https://github.com/casdoor/casdoor/issues/6078
- PR: https://github.com/casdoor/casdoor/pull/6079
- Pinned fix commit: `4af4cea24a766e97757963daf36c59d02627aeb8`

The one-line PR diff is stored in `patches/pr-6079.patch`; builds do not follow
future edits to the PR branch.

## Setup

1. This build runs from the existing `VincentSchmid/casdoor` fork. Its workflow
   checks out the latest upstream stable release separately, so the fork's source
   does not need to be synchronized for image builds.
2. Open **Actions > Build patched Casdoor > Run workflow**, or let changes to the
   workflow or patch on `master` trigger the build.
3. After the first successful build, open the package on GitHub and change its
   visibility to **Public** if you want deployments to pull without credentials.
   Otherwise configure your deployment with GHCR pull credentials.

No extra secrets are needed. The workflow publishes using `GITHUB_TOKEN` with
`packages: write`. Organization policies must allow Actions and package publishing.
Use a fresh package name, or grant this repository Actions access to an existing
package of the same name.

## Published images

The image is named `casdoor`, regardless of the GitHub repository name. The owner
is lowercased for GHCR:

```text
ghcr.io/YOUR_OWNER/casdoor:v4.27.1
```

Only the upstream release tag is published, without a PR suffix or `latest` tag.
The tag above is an example; each run resolves GitHub's latest stable Casdoor
release, not the unreleased `master` branch or an older fixed version.
Images target only `linux/arm64` for Raspberry Pi 5 running a 64-bit OS, using
upstream's `STANDARD` Dockerfile target (not the all-in-one database image).
Builds run on GitHub's native `ubuntu-24.04-arm` runner without QEMU emulation.

The workflow runs daily at approximately 05:23 UTC, on changes to its build files
on `master`, and on demand. It rebuilds even if no new release is available, so it
can refresh upstream base images. The version tag can therefore change; the build
summary includes an immutable image digest for reproducible deployments.

If upstream already contains the patch, the workflow builds without reapplying
it. If neither the forward nor reverse patch applies, it fails without publishing
rather than guessing how to modify authentication code.

## Deployment

Use a 64-bit OS on the Pi, such as Raspberry Pi OS 64-bit or Ubuntu ARM64.
`uname -m` should report `aarch64`; a 32-bit OS cannot run this image.

Change only the image reference in your existing deployment. Keep your database,
configuration, and persistent storage. Back up the database and test the affected
CIMD sign-in flow before production; newly issued tokens should use your
organization's certificate `kid` and validate against its expected JWKS.

Publishing an image does not automatically update running containers.
Redeploy explicitly, or configure your existing deployment automation to do so.

Scheduled builds use GitHub Actions minutes. GitHub can delay scheduled runs and
disables schedules in public repositories after 60 days without repository
activity; re-enable the workflow if that happens.

## Update or retire the patch

Review and replace the vendored patch if the PR changes. Once an upstream release
contains the fix, you can switch back to the official Casdoor image and retire
this repository.
