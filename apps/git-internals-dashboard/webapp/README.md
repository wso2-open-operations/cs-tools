# Git Internals Dashboard Webapp

SLA-monitor dashboard SPA — React 19 + TypeScript + Vite, `@wso2/oxygen-ui`, TanStack Query. Talks
to `backend/`'s API; no business logic lives here.

## Tech Stack

- **Core**: [React 19](https://react.dev/)
- **Build Tool**: [Vite](https://vitejs.dev/)
- **Language**: [TypeScript](https://www.typescriptlang.org/)
- **UI Library**: [Oxygen UI](https://github.com/wso2/oxygen-ui) (`@wso2/oxygen-ui`, MUI-based)
- **Routing**: [react-router](https://reactrouter.com/) v7
- **Data Fetching**: [TanStack Query](https://tanstack.com/query/latest)
- **Charts**: [Recharts](https://recharts.org/)
- **Authentication**: [Asgardeo](https://wso2.com/asgardeo/) (`@asgardeo/react` + `@asgardeo/react-router`)
- **Testing**: [Vitest](https://vitest.dev/) & [React Testing Library](https://testing-library.com/docs/react-testing-library/intro/)

## Getting Started

### Prerequisites

- Node.js 24 (LTS)
- pnpm 10
- `backend/` running locally (see [`backend/README.md`](../backend/README.md)) — this app has
  nothing to render without it

### Installation

```bash
pnpm install
cp public/config.js.example public/config.js
```

Edit `public/config.js` — at minimum, point `GID_BACKEND_BASE_URL` at your running backend
(`http://localhost:8080` for local dev). The Asgardeo keys only matter once you have a real
Asgardeo SPA application to sign in against.

### Development

```bash
pnpm dev     # :5173
```

### Build

```bash
pnpm build     # tsc -b && vite build, output to dist/
pnpm preview   # serve the production build locally
```

## Configuration

Runtime config is read from `window.config`, set by `public/config.js` (git-ignored — never
commit your real one). `src/config/windowConfig.ts` is the single typed accessor; nothing else in
the app reads `window.config` directly.

| Key | Description | Example |
|---|---|---|
| `GID_AUTH_BASE_URL` | OIDC IdP base URL for the configured Asgardeo organization | `https://api.asgardeo.io/t/<org>` |
| `GID_AUTH_CLIENT_ID` | OAuth2 client id (SPA/PKCE — no client secret) | `<client-id>` |
| `GID_AUTH_SIGN_IN_REDIRECT_URL` | Sign-in callback URL | `http://localhost:5173` |
| `GID_AUTH_SIGN_OUT_REDIRECT_URL` | Sign-out callback URL | `http://localhost:5173` |
| `GID_AUTH_SCOPES` | OIDC scopes, space-separated | `openid profile` |
| `GID_BACKEND_BASE_URL` | Backend API base URL — no `/api` prefix | `http://localhost:8080` |
| `GID_LOG_LEVEL` | Reserved for future console log gating | `ERROR` |

### Import Aliases

Use the `@`-prefixed aliases (`@api`, `@components`, `@config`, `@features`, `@lib`, `@layouts`,
`@theme`) instead of relative imports beyond one level. Defined in `vite.config.ts`, mirrored in
`tsconfig.app.json`.

## Testing

```bash
pnpm test    # vitest run
pnpm lint    # eslint .
```

## Deploying

Hosted as a Web Application (no `.choreo/component.yaml` here — same as the backend's Choreo
Service, but this half needs no schema). Buildpack React, build command
`pnpm install && pnpm build`, output directory `dist`, Node 24. `config.js` is supplied
per-environment via a config-mount at `/config.js` — the same mechanism `public/config.js` fills
locally, so the built bundle itself never changes between environments.
