# Releasing

Use the release workflows from `main`.

## Prerequisites

- GitHub Actions publish workflows are enabled.
- `RELEASE_TOKEN` is configured with permission to push release commits and tags.
- npm publishing is configured for `@ssubedir/open-spanner`.
- PyPI publishing is configured for `open-spanner`.
- NuGet publishing is configured for `OpenSpanner`.

## Prepare Version

Open the `Prepare Release Version` workflow from `main` and enter the release version, for example `0.1.6`.

The workflow runs:

```sh
task release:version VERSION=0.1.6
```

It commits the SDK package metadata directly to `main`.

## Release

Open the `Release Orchestrator` workflow from `main` and enter the same release version.

The workflow:

- runs `task release:check`
- verifies SDK package versions match the release version
- creates the app and SDK release tags in order

Those tags trigger the GitHub release and SDK publish workflows.

## Optional Local Preflight

Before running the orchestrator, you can run:

```sh
task release:check
```

If generated files change, commit them to `main` and rerun the check.

## Publish Targets

| Package | Registry | Workflow |
| --- | --- | --- |
| Open Spanner | GitHub Releases | `Create GitHub Release` |
| TypeScript | npm `@ssubedir/open-spanner` | `Publish TypeScript SDK` |
| Python | PyPI `open-spanner` | `Publish Python SDK` |
| C# | NuGet `OpenSpanner` | `Publish C# SDK` |
| Go | Go module proxy | `Release Orchestrator` |

## Failed Releases

If the release check fails, fix and commit the reported files on `main`, then rerun `Release Orchestrator`.

If a registry rejects a package because the version already exists, prepare the next version and run the release again.
