# Contributing to Open Spanner

Product documentation lives in `docs/content/docs`. Keep that site focused on
evaluating, integrating, using, and operating Open Spanner. Repository workflow
and generator details belong here.

## Local Development

Install [Task](https://taskfile.dev/), then use the commands relevant to your
change:

```sh
task run:sqlite
task control-plane:dev
task test
task test:postgres
task test:s3
task test:e2e
task docs:build
```

Run `task test:load` for the Postgres ingestion regression profile or
`task test:load:soak` for the longer soak profile. The harness verifies
throughput, p95 latency, idempotent replay, persisted-event integrity, recovery
after uncertain stream writes, and bounded telemetry cardinality.

## Generated Contracts And SDKs

Regenerate API and SDK inputs with:

```sh
task openapi:sdk
```

Check individual SDKs with:

```sh
task sdk:go:check
task sdk:typescript:check
task sdk:python:check
task sdk:csharp:check
```

Generate and check protobuf code with `task proto` and `task proto:check`.
Open Spanner uses Buf remote plugins, so a local `protoc` installation is not
required.

Before opening a pull request, run the smallest relevant checks plus the
cross-cutting integration suite for behavior you changed.
