# Open Spanner Web

The Open Spanner dashboard is a Next.js App Router application. Source files live
under `src/`, file-based routes are defined in `src/app/`, and reusable
shadcn-style primitives live in `src/components/ui/`.

## Development

From the repository root, start the API and then run the dashboard:

```sh
task run:sqlite
task admin:dev
```

The dashboard is available at
[http://127.0.0.1:18081/overview](http://127.0.0.1:18081/overview). It proxies
`/v1` requests to the API configured by `OPEN_SPANNER_API_PROXY_URL`.

To run Next.js directly from this directory:

```sh
npm ci
npm run dev
```

By default, the direct Next.js command listens on
[http://localhost:3000](http://localhost:3000) and proxies API requests to
`http://127.0.0.1:18080`.

## Checks

```sh
npm run lint
npm run typecheck
npm run build
npm audit
```

Run the full Playwright suite from the repository root with:

```sh
task test:e2e:web
```
