# Releasing

Use this checklist before publishing SDK packages or cutting a tagged release.

## Prerequisites

- GitHub Actions publish workflows are enabled.
- npm package publishing is configured for `@ssubedir/open-spanner`.
- PyPI trusted publishing is configured for `open-spanner`.
- NuGet trusted publishing is configured for `OpenSpanner`.
- Local tools are installed: Go, Node/npm, Python/uv, .NET SDK, and Task.

## Verify

Run the full release gate from a clean working tree:

```sh
task release:check
```

This regenerates OpenAPI and SDK artifacts, runs tests, builds package artifacts, and fails if generated files are not committed.

If the command reports a diff, review it and commit the generated artifacts before tagging:

```sh
git status --short
git diff --stat
```

Then rerun:

```sh
task release:check
```

## Version Updates

Update package versions before tagging:

- TypeScript: `sdk/typescript/package.json`
- Python: `sdk/python/pyproject.toml`
- C#: `sdk/csharp/OpenSpanner.csproj`

Regenerate and verify after version changes:

```sh
task release:check
```

## Tags

SDK publish workflows are tag driven:

```sh
git tag sdk-js-v0.1.1
git tag sdk-python-v0.1.1
git tag sdk-csharp-v0.1.1
git push origin sdk-js-v0.1.1 sdk-python-v0.1.1 sdk-csharp-v0.1.1
```

For the Go SDK, use a module tag when publishing a new Go module version:

```sh
git tag sdk/go/v0.1.1
git push origin sdk/go/v0.1.1
```

## Package Targets

| SDK | Registry | Workflow | Tag |
| --- | --- | --- | --- |
| TypeScript | npm `@ssubedir/open-spanner` | `Publish TypeScript SDK` | `sdk-js-v*` |
| Python | PyPI `open-spanner` | `Publish Python SDK` | `sdk-python-v*` |
| C# | NuGet `OpenSpanner` | `Publish C# SDK` | `sdk-csharp-v*` |
| Go | Go module proxy | none | `sdk/go/v*` |

## Failed Release Checks

If a publish workflow fails because generated files changed, do not rerun the failed job blindly. Pull the branch locally, run:

```sh
task release:check
```

Commit the generated OpenAPI or SDK changes, push the commit, then recreate or push a new release tag.

If a registry rejects a package because the version already exists, bump the package version and create a new tag.
