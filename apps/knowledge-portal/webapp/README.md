# Knowledge Portal webapp

React + TypeScript + Vite frontend for the public knowledge portal, built on
`@wso2/oxygen-ui`, TanStack React Query and `react-router` v7. It is read-only
and has no authentication: no login, no tokens, no protected routes.

It follows the layout and conventions of the sibling webapps (`csm-portal`,
`customer-portal`): path aliases (`@api`, `@components`, `@config`, `@features`,
`@hooks`, `@layouts`, `@utils`), a typed API client that throws an error carrying
the HTTP status, runtime configuration through `public/config.js`, and Vitest for
unit and component tests.

## Configuration

There is no build-time configuration. The backend URL is read at runtime from
`window.config`, set by `public/config.js` and loaded by a `<script>` tag before
the app boots, so one build serves every environment.

```bash
cp public/config.js.example public/config.js   # then edit it
```

| Key | Required | Description |
|---|---|---|
| `KNOWLEDGE_PORTAL_BACKEND_BASE_URL` | yes | Absolute http(s) URL of the knowledge portal backend. CORS is set by the API gateway, not by the backend. |
| `KNOWLEDGE_PORTAL_SUPPORT_URL` | no | Absolute http(s) URL for the "Contact support" callouts on the home, browse and article pages. Unset hides them; a malformed value is ignored with a console warning. |

There is deliberately no default for the backend URL. If the key is missing or
malformed the app shows a message naming it instead of calling a guessed address.

In Choreo, serve `config.js` as a file mount at `/usr/share/nginx/html/config.js`.

`config.js` and any `config.*.js` or `.env` file are git-ignored and must never
be committed. Only `public/config.js.example` is tracked.

## Features

- **Search with suggestions.** Every search box offers the top matching article
  titles while typing; Enter runs the full search, arrow keys and Enter open a
  suggestion. Pressing `/` anywhere on the page focuses the search box.
- **Home.** Product cards with article counts, recently published articles and
  quick links to the most common knowledge bases.
- **Browse.** Knowledge-base filter (sidebar on wide screens, chips on narrow),
  matched terms highlighted in results, exact counts per knowledge base.
- **Article.** "On this page" outline that tracks the section being read, copy
  buttons on code samples, copy-link and print actions, reading time, related
  articles and a print stylesheet.
- **Support callout.** Shown wherever a reader may have reached a dead end,
  only when `KNOWLEDGE_PORTAL_SUPPORT_URL` is configured.

## Commands

| Task | Command |
|---|---|
| Dev server (port 3100) | `pnpm run dev` |
| Type check | `pnpm run typecheck` |
| Lint | `pnpm run lint` |
| Tests (single run) | `pnpm run test` |
| Production build | `pnpm run build` |
