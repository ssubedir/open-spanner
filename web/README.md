# Open Spanner Web UI

React dashboard for Open Spanner. The UI is built with Vite, React, TanStack Router, react-querybuilder, and local shadcn-style components.

## Pages

- `/overview` - system totals, subject activity, and ingestion history
- `/meters` - create, list, edit, and delete meter definitions
- `/usage` - create usage events and query bucketed usage with advanced filters

`/` redirects to `/overview`. The Go API embeds the built UI and serves these routes from the same origin as the `/v1` API.

## Development

Install dependencies:

```sh
npm install
```

Run the Vite dev server:

```sh
npm run dev
```

From the repository root, the same command is available through Task:

```sh
task admin:dev
```

The app uses relative `/v1/...` API calls. For full integration testing, run the Go API with the built UI so the dashboard and API share the same origin.

## Build

Build the embedded UI assets:

```sh
npm run build
```

From the repository root:

```sh
task admin:build
```

The build output is written to `internal/ui/static` for Go embedding. The build script removes stale asset files before Vite writes the new bundle.

## Checks

```sh
npm run lint
npm run build
```

## Notes

- Keep route paths in sync with `internal/ui/ui.go`.
- Keep API calls in `src/api.ts` relative unless the backend serving model changes.
- Advanced usage filtering is powered by `react-querybuilder` and maps to the `/v1/usages/search` request shape.
