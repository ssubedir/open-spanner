# Open Spanner Docs Site

Documentation site for Open Spanner, built with Next.js and Fumadocs.

This app is intentionally separate from the repository-level `openapi/` folder, which contains generated Swagger/OpenAPI artifacts.

## Development

```sh
npm install
npm run dev
```

Open `http://localhost:3000`.

From the repository root:

```sh
task docs:dev
```

## Build

```sh
npm run build
```

This app is configured for static export. The output is written to `out/`.

From the repository root:

```sh
task docs:build
```

## GitHub Pages

The repository deploys this app to GitHub Pages from `.github/workflows/deploy-docs.yml`.

The workflow builds with:

```sh
NEXT_PUBLIC_BASE_PATH=/open-spanner NEXT_PUBLIC_SITE_URL=https://ssubedir.github.io/open-spanner npm run build
```

Use the default local commands for development; the base path is only needed for the GitHub Pages project URL.

## Content

- `content/docs` contains MDX pages.
- `source.config.ts` configures Fumadocs MDX collections.
- `src/lib/source.ts` loads the generated collection for layouts, search, and LLM routes.
