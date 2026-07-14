# Version and Release Design

The implemented version and release design is maintained in:

- [`docs/VERSIONING.md`](../docs/VERSIONING.md) for the project identity contract;
- [`docs/RELEASE.md`](../docs/RELEASE.md) for the executable release and recovery flow;
- [`docs/DEVELOPMENT.md`](../docs/DEVELOPMENT.md) for shared quality gates and
  release-tool testing.

The current design requires a same-configuration pre-tag production candidate,
an immutable annotated version tag, a temporary detached worktree at that exact
tag for final artifacts, clean source checks before and after the build, and a
manifest cross-validated against Git, the DMG, the app bundle, code-signing,
notarization, and installation results.

Do not duplicate the detailed contract in this specification; update the
authoritative documents and executable scripts together.
