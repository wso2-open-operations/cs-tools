# CSM Portal Webapp

Internal customer-success-manager portal for WSO2. Built as a React 19 + TypeScript + Vite SPA. The codebase is scaffolded from `apps/customer-portal/webapp` and currently runs the same routes; CSM-native features (cross-customer cases queue, accounts list, time-cards) are added alongside under a "view-as-customer" model.

## Tech Stack

- **Core**: [React 19](https://react.dev/)
- **Build Tool**: [Vite](https://vitejs.dev/)
- **Language**: [TypeScript](https://www.typescriptlang.org/)
- **UI Library**: [Oxygen UI](https://github.com/wso2/oxygen-ui)
- **Data Fetching**: [TanStack Query](https://tanstack.com/query/latest) (React Query)
- **Authentication**: OIDC against the configured identity provider (any standard OIDC IdP is supported)
- **Testing**: [Vitest](https://vitest.dev/) & [React Testing Library](https://testing-library.com/docs/react-testing-library/intro/)

## Getting Started

### Prerequisites

- [Node.js](https://nodejs.org/) (Latest LTS recommended)
- [pnpm](https://pnpm.io/) (Recommended package manager)

### Installation

1. Install dependencies:

   ```bash
   pnpm install
   ```

2. Copy the runtime config template:

   ```bash
   cp public/config.js.example public/config.js
   ```

3. Update `public/config.js` with the CSM portal's IdP SPA application details and backend base URL. The CSM portal uses a separate IdP SPA application from the customer portal.

### Development

Dev server defaults to port `3001` (customer-portal uses 3000):

```bash
pnpm run dev
```

### Build

```bash
pnpm run build
```

Preview the production build:

```bash
pnpm run preview
```

## Configuration

Runtime config is read from `window.config` set by `public/config.js`. Build-time env vars are prefixed `CSM_PORTAL_` (see `vite.config.ts`).

### Runtime Config Keys (public/config.js)

| Key | Description | Example |
|---|---|---|
| `CSM_PORTAL_AUTH_BASE_URL` | OIDC IdP base URL for the CSM SPA application | `<your-idp-base-url>` |
| `CSM_PORTAL_AUTH_CLIENT_ID` | OAuth2 Client ID (CSM SPA application, NOT the customer-portal one) | `<client-id>` |
| `CSM_PORTAL_AUTH_SIGN_IN_REDIRECT_URL` | Sign-in callback URL | `http://localhost:3001` |
| `CSM_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL` | Sign-out callback URL | `http://localhost:3001` |
| `CSM_PORTAL_BACKEND_BASE_URL` | Backend API base URL (CSM BFF) | `http://localhost:9090` |
| `CSM_PORTAL_THEME` | Theme (acrylicOrange, acrylicPurple, highContrast, classic) | `acrylicOrange` |
| `CSM_PORTAL_LOG_LEVEL` | Logging level (DEBUG, INFO, WARN, ERROR) | `INFO` |

### Import Aliases

Use `@`-prefixed aliases instead of relative imports beyond one level. Defined in `vite.config.ts`.

## Logging

```typescript
const logger = useLogger();
logger.info("Message to log");
```

Logger respects `CSM_PORTAL_LOG_LEVEL` from runtime config.

## Testing

```bash
pnpm run test          # Vitest
pnpm run test:e2e      # Playwright
```

## Branching

This webapp lives in the `cs-tools` repo. Work on a feature branch off `v2` (never commit directly to `v2`). Rebase on `origin/v2` before opening a PR.
