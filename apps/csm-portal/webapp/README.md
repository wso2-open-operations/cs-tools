# CSM Portal Webapp

The browser front end of the CSM portal: the internal application customer-success engineers use to work cases, operations records (service requests, change requests, incidents, problems), engagements, time cards, announcements, customers/projects and user administration. It is a single-page app that talks only to the CSM portal backend (a BFF) over HTTPS; it never calls the entity service or any data source directly.

For how this webapp fits into the wider platform (customer portal, backends, entity services, database), see the [platform architecture](../README.md#architecture). The rest of this document covers the webapp only.

**Contents**

- [Architecture](#architecture)
- [Developer guide](#developer-guide)
- [Configuration](#configuration)
- [Local full-stack mode (sandboxed dev)](#local-full-stack-mode-sandboxed-dev)

---

## Architecture

### Tech stack

| Concern | Choice |
|---|---|
| Framework | [React 19](https://react.dev/) with the React Compiler (`babel-plugin-react-compiler`) |
| Language | [TypeScript](https://www.typescriptlang.org/) |
| Build tool | [Vite](https://vitejs.dev/) (dev server on port `3001`) |
| Routing | [`react-router`](https://reactrouter.com/) v7 (the unified `react-router` package, not `react-router-dom`), declarative `<BrowserRouter>` / `<Routes>` |
| Server state | [TanStack Query](https://tanstack.com/query/latest) (React Query) |
| Client state | React context providers under `src/context/` (current user, theme, logger, error/success banners, case tabs); no global store library |
| UI library | [Oxygen UI](https://github.com/wso2/oxygen-ui) (`@wso2/oxygen-ui`, built on MUI), plus its icon and chart packages |
| Auth | OIDC authorization-code + PKCE against a configurable identity provider, through a React OIDC SDK |
| Rich text / content | Lexical (editor), `markdown-it` / `react-markdown` (render), DOMPurify (sanitise) |
| Unit/component tests | [Vitest](https://vitest.dev/) + [React Testing Library](https://testing-library.com/docs/react-testing-library/intro/) (jsdom) |
| End-to-end tests | [Playwright](https://playwright.dev/) |

### Source layout

```
src/
  main.tsx            Entry point: boot guards (deep-link capture, stale-chunk reload), mounts <AppWithConfig/>
  AppWithConfig.tsx   Provider stack: OIDC provider, router, logger, theme, TanStack Query client, error boundary
  App.tsx             Route table (<Routes>) and page imports
  api/                Cross-feature API plumbing
    backend/          client.ts (useBackendApi), shared request/response types, mappers
  components/         Reusable, feature-agnostic UI (chips, selects, banners, rich-text editor, error states, ...)
  config/             Reads window.config into typed settings; nav tree (csmNavItems.ts); feature flags; theme registry
  constants/          Shared constants (API, auth/idle timings)
  context/            React contexts/providers (current-user, theme, logger, banners, case tabs)
  features/           One folder per product area (see below)
  hooks/              Cross-feature hooks (auth fetch, token access, logger, column preferences, ...)
  layouts/            App shell, AuthGuard, error layout, post-login redirect
  providers/          Cross-cutting providers (idle timeout)
  styles/, assets/, types/, utils/
tests/e2e/            Playwright specs, page objects, fixtures
public/               Static assets and config.js.example
```

Tests are co-located with the code they cover (`Foo.tsx` next to `Foo.test.tsx`).

### Feature modules (`src/features/*`)

Each product area is a self-contained folder, usually `csm-<area>` (for example `csm-cases`, `csm-timecards`, `csm-operations`, `csm-admin`, `csm-announcements`) plus a few shared ones (`help`, `settings`, `updates`, `case-tabs`). A feature typically contains only the sub-folders it needs:

```
src/features/csm-example/
  api/          TanStack Query hooks (useGetThing, usePatchThing, ...) built on useBackendApi
  components/   Components used only by this feature
  pages/        Route-level components (what App.tsx renders)
  types/        Types for this feature's payloads
  utils/        Pure helpers
  hooks/, constants/   As needed
```

Conventions:

- Features import from shared layers (`@components`, `@hooks`, `@api`, `@utils`, ...). Avoid importing one feature's internals from another where a shared module would do.
- Data access goes through hooks in the feature's `api/` folder, never `fetch` calls inside components.
- Use the `@`-prefixed import aliases (below) instead of relative imports that climb more than one level.

### Routing and navigation

- Routes are declared in `src/App.tsx`. Pages are imported eagerly (no per-route code splitting), so navigating never triggers a runtime chunk fetch.
- Authenticated routes sit under `<AuthGuard>` (`src/layouts/AuthGuard.tsx`) and a `FeatureRouteGuard`; the unauthenticated error routes (`/401`, `/403`, `/404`) sit outside.
- The sidebar, tab strips, quick-nav and Help topics are all driven by one navigation tree in `src/config/csmNavItems.ts`. Each node has a stable dotted id (`operations`, `operations.incidents`). These ids are a public contract: deployments use them as keys in `CSM_PORTAL_FEATURE_OVERRIDES` (see [Configuration](#configuration)), so renaming one is a breaking configuration change.
- `src/config/featureFlags.ts` resolves a node to `enabled`, `wip` (visible but greyed out, routes render a "coming soon" page) or `hidden` (no nav entry, routes redirect to the dashboard).

### API client layer

- All backend calls go through `useBackendApi()` in `src/api/backend/client.ts`. It returns thin typed wrappers (`get`, `post`, `patch`, `put`, `del`, `delete`, `postEmpty`, `getBlob`) that join the path onto `CSM_PORTAL_BACKEND_BASE_URL` and JSON-encode the body. A `GET` that returns `404` resolves to `null`; any other non-2xx response throws a `BackendApiError` carrying the HTTP status, the backend's error payload and a correlation id.
- Under the hood it uses `useAuthApiClient()` (`src/hooks/useAuthApiClient.ts`), an authenticated `fetch` wrapper described below.
- Every request carries an `X-CSM-Correlation-ID` header; the backend echoes it, and the UI surfaces it as a support reference id on errors.
- Data hooks wrap these calls in TanStack Query. The shared `QueryClient` (in `AppWithConfig.tsx`) retries a failed query at most twice and only for `502`/`503`; mutations never retry; refetch-on-window-focus is off.
- Case activity can optionally be pushed over a server-sent-events stream (`CSM_PORTAL_STREAM_BASE_URL` plus `CSM_PORTAL_STREAM_ENABLED`); without it the UI falls back to query polling/manual refresh.

### Authentication and session handling

- The app signs users in with an OIDC authorization-code + PKCE flow against the identity provider configured in `window.config` (`CSM_PORTAL_AUTH_*`). Any standards-compliant OIDC provider can be used. Requested scopes: `openid email groups profile roles`. The webapp is IdP-agnostic and never handles credentials.
- Token storage and refresh are owned by the OIDC SDK, which keeps its session in browser storage. Application code does not persist tokens itself.
- `useAuthApiClient` attaches the access token as `Authorization: Bearer <token>` and the ID token as `x-user-id-token`. The API gateway validates the bearer token at the edge; the backend identifies the caller from the validated identity. **Tokens are only attached when the request origin equals the configured backend origin**; any other origin is refused so credentials cannot leak to third-party hosts.
- On a `401` or an expired token the client recovers in order: retry (another caller may have refreshed), silent re-authentication in a hidden iframe, then a full sign-in redirect as the last resort (guarded against redirect loops). Deep links survive the round trip through the IdP via a session-storage "post-login redirect" key.
- `AuthGuard` calls `GET /users/me` before showing any page. If that fails with `401`/`403`, the user sees a "no access to this portal" page and no routed page is mounted.
- An idle timer (`IdleTimeoutProvider`) shows a "still there?" prompt after 15 minutes of inactivity (30 seconds before the deadline); Continue resets it, Log out ends the session.

**No frontend authorization gates.** The platform authorizes on the server: the backend decides what a caller may do, not checks made in the browser. The UI therefore shows actions and lets the backend reject them, then surfaces the error (for example through the error banner). `usePortalAccess()` exists so controls the caller certainly cannot use do not clutter the UI; it is a convenience, never the security boundary, and the backend's `403` is authoritative. Deliberate exceptions are rare and documented in [`CLAUDE.md`](./CLAUDE.md); when adding an action, default to show-and-let-the-backend-reject.

### Theming

- Themes come from Oxygen UI. `src/config/themeConfig.ts` builds the registry from whatever theme objects the installed `@wso2/oxygen-ui` version exports, so a new upstream theme appears in the picker without code changes.
- The default is `CSM_PORTAL_THEME`; users can switch from the header theme picker, and the choice is persisted per browser (`localStorage` key `csm.theme`).
- Accessibility overrides live in `src/config/a11yThemeOverrides.ts`.

### In-app user guide (`/help`)

The `/help` route is the user guide shown to portal users. Its content is plain Markdown in `src/features/help/content/`, one file per navigation section (`support.md`, `time-cards.md`, ...). The files are bundled at build time through Vite's `?raw` import; no backend is involved.

Keep the guide in step with the UI: whenever a change adds, moves, renames or removes a user-facing flow, field or navigation item, update the matching topic in the same pull request.

To add a topic:

1. Create `src/features/help/content/<topic-id>.md`.
2. Import it with `?raw` and add it to `HELP_TOPIC_CONTENT` in `src/features/help/utils/helpContent.ts`, keyed by `<topic-id>`.
3. Add a `help.<topic-id>` child (label and `href: "/help#<topic-id>"`) under the `help` node in `src/config/csmNavItems.ts`; that node decides which topics exist and their order.
4. Run `pnpm exec vitest run src/features/help`. `helpContent.test.ts` fails if a nav topic has no content entry.

### Import aliases

Defined in `vite.config.ts` and mirrored in `tsconfig.app.json`: `@` (`src`), `@api`, `@assets`, `@components`, `@config`, `@constants`, `@context`, `@features`, `@hooks`, `@layouts`, `@providers`, `@utils`. A few narrower aliases (`@case-details*`, `@time-tracking`, `@deployments`, `@update-cards`) point into specific feature sub-folders; do not add more of that kind. Use aliases instead of relative imports that climb more than one level.

---

## Developer guide

### Prerequisites

- [Node.js](https://nodejs.org/) 20.19+ or 22.12+ (the minimum Vite 7 supports); a current LTS is recommended. The repository's end-to-end workflow runs on Node 24.
- [pnpm](https://pnpm.io/) 10 or later. The repo uses `pnpm-lock.yaml`; do not use npm or yarn here. Note that `package.json` declares no `engines` or `packageManager` field, so these versions come from the toolchain, not an enforced pin.
- A running CSM portal backend and an OIDC identity provider SPA application to sign in against, or use [local full-stack mode](#local-full-stack-mode-sandboxed-dev).

### Install

```bash
pnpm install
cp public/config.js.example public/config.js   # then edit it, see Configuration
```

### Run, build, check

| Task | Command | Notes |
|---|---|---|
| Dev server | `pnpm run dev` | Port `3001` (`strictPort`, fails if busy) |
| Local-gateway dev server | `pnpm run dev:local` | See [local full-stack mode](#local-full-stack-mode-sandboxed-dev) |
| Production build | `pnpm run build` | Runs `tsc -b` (type check) then `vite build` |
| Preview the build | `pnpm run preview` | Serves `dist/` |
| Lint | `pnpm run lint` | ESLint 9 flat config (`eslint.config.js`) |
| Unit/component tests | `pnpm run test` | Vitest in watch mode; use `pnpm exec vitest run` for a single run |
| E2E tests | `pnpm run test:e2e` | Playwright, see below |
| E2E session capture | `pnpm run test:e2e:auth` | Mints a signed-in session for the e2e specs |

Run `pnpm run lint`, `pnpm exec vitest run` and `pnpm run build` before opening a pull request.

### Testing

- **Unit/component**: Vitest with jsdom and React Testing Library, `*.test.ts(x)` files next to the source. The Playwright tree (`tests/e2e/**`) is excluded from Vitest. Mock the OIDC hooks (`vi.mock`) where a test needs a signed-in user.
- **End to end**: Playwright specs in `tests/e2e/`, run against the dev server and an authenticated session captured beforehand. Read [`tests/e2e/README.md`](./tests/e2e/README.md) for what the specs cover and how to run them, and [`tests/e2e/auth/README.md`](./tests/e2e/auth/README.md) for capturing a session. Session files are git-ignored because they hold real tokens; never commit them. By default Playwright starts `pnpm run dev` itself (reusing a running one locally); set `E2E_NO_WEBSERVER` to skip that and `E2E_BASE_URL` to target a different origin.

### Adding a page or feature

1. Create or choose a feature folder `src/features/csm-<area>/` with `api/`, `components/`, `pages/` as needed.
2. Add data hooks in `api/` using `useBackendApi()` inside TanStack Query `useQuery` / `useMutation`. Call only endpoints that exist in the backend's published contract; do not invent new ones from the front end.
3. Build the page component in `pages/` with Oxygen UI components.
4. Register the route in `src/App.tsx` inside the authenticated `<AuthGuard>` / `FeatureRouteGuard` block.
5. If it should appear in navigation, add a node to `src/config/csmNavItems.ts` with a stable dotted id (and an optional `requires` capability if it should be hidden for users lacking it). Remember the id doubles as a feature-override key.
6. Show actions regardless of the caller's role and surface backend errors; do not add authorization checks in the UI.
7. Write tests next to the code, and update the matching `/help` topic if the change is user-visible.

### Code conventions

- Functional components and hooks only; TypeScript strict-style typing, no `any` shortcuts.
- Every source file carries the Apache 2.0 licence header used throughout `src/`.
- Read configuration through the typed modules in `src/config/`, not by touching `window.config` from components.
- Server state belongs in TanStack Query, not in context or local state copies.
- Keep the app vendor-neutral: no identity-provider or data-source vendor names in code, comments, config examples or docs.
- Use the logger rather than `console`:

  ```typescript
  const logger = useLogger();
  logger.info("Message to log");
  ```

  The level comes from `CSM_PORTAL_LOG_LEVEL` (defaults to `ERROR` when unset).

---

## Configuration

### How it works

The webapp has no build-time environment configuration of its own. Everything environment-specific is read at runtime from `window.config`, which is set by `public/config.js`, loaded by a plain `<script src="/config.js">` tag in `index.html` before the app boots. The same build artifact can therefore be deployed to many environments by serving a different `config.js`. At build time the tag gets a `?v=<git short sha or timestamp>` suffix so CDNs fetch a fresh copy after each deploy.

`window.config` is typed in `src/config/authConfig.ts`; each key is consumed by the module named in the table below. The required auth and backend keys are checked at start-up and the app throws a clear error naming the missing key. (Vite is also configured with `envPrefix: ["CSM_PORTAL_"]`, but no application code reads `import.meta.env` values for configuration, so use `config.js`.)

**Real config files are git-ignored and must never be committed.** `public/config.js`, any `config.*.js`, and any `.env` file carry endpoints and client identifiers for real environments. Only the template is tracked:

```bash
cp public/config.js.example public/config.js
```

Edit your copy with values for your environment. The template (`public/config.js.example`) holds the authoritative, commented example for every key, including banner and feature-override samples. The examples below are placeholders only.

### Required keys

| Key | Description | Example |
|---|---|---|
| `CSM_PORTAL_AUTH_BASE_URL` | Base URL of the OIDC identity provider | `https://idp.example.com` |
| `CSM_PORTAL_AUTH_CLIENT_ID` | OAuth2 client id of the identity provider SPA application registered for this portal (use a separate application from any other portal) | `your-client-id` |
| `CSM_PORTAL_BACKEND_BASE_URL` | Base URL of the CSM portal backend (BFF). Bearer tokens are only ever sent to this origin | `https://api.example.com/csm-portal/v1.0` |

### Optional keys

| Key | Type / default | Description | Example |
|---|---|---|---|
| `CSM_PORTAL_AUTH_SIGN_IN_REDIRECT_URL` | string, page origin | URL the IdP redirects to after sign-in. Unset means the page's own origin (`window.location.origin`), so the app works on any host and port; that origin must be a registered redirect URI on the IdP application. Set it only to override, e.g. behind a proxy whose public origin differs from the browser's | `http://localhost:3001` |
| `CSM_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL` | string, page origin | URL the IdP redirects to after sign-out. Same default and override rules as the sign-in URL; must also be registered | `http://localhost:3001` |
| `CSM_PORTAL_THEME` | string, `acrylicOrange` | Default theme key. The valid set is derived from the installed Oxygen UI version (for example `acrylicOrange`, `acrylicPurple`); users can override it from the header and the choice persists per browser | `acrylicOrange` |
| `CSM_PORTAL_LOG_LEVEL` | string, `ERROR` | Logger level: `DEBUG`, `INFO`, `WARN`, `ERROR` | `INFO` |
| `CSM_PORTAL_FEATURE_OVERRIDES` | object or JSON string, `{}` | Per-page visibility keyed by navigation-node id (a section such as `operations`, or a tab such as `operations.incidents`). Values: `enabled`, `wip`, `hidden`. Unlisted pages are enabled; a section's value cascades to its tabs unless the tab overrides it; a `hidden` section hides all its tabs. Unknown ids are ignored with a console warning. The available ids are the `id` fields in `src/config/csmNavItems.ts` | `{ "operations": "wip", "operations.incidents": "enabled" }` |
| `CSM_PORTAL_STREAM_BASE_URL` | string, unset | Base URL of the case-activity server-sent-events stream exposed by the backend. Leave unset if the backend has no stream | `https://stream.example.com/csm-portal-stream/v1.0` |
| `CSM_PORTAL_STREAM_ENABLED` | boolean, `false` | Master switch for the stream. A live connection is only opened when this is `true` **and** `CSM_PORTAL_STREAM_BASE_URL` is set | `false` |
| `CSM_PORTAL_MAINTENANCE_BANNER_VISIBLE` | boolean, `false` | Show the notification/maintenance banner | `false` |
| `CSM_PORTAL_MAINTENANCE_BANNER_SEVERITY` | string, `info` | `info`, `warning`, `error` or `success` (anything else falls back to `info`) | `warning` |
| `CSM_PORTAL_MAINTENANCE_BANNER_TITLE` | string, empty | Banner title | `Scheduled maintenance` |
| `CSM_PORTAL_MAINTENANCE_BANNER_MESSAGE` | string, empty | Banner body text | `Some services may be unavailable.` |
| `CSM_PORTAL_MAINTENANCE_BANNER_ACTION_LABEL` | string, unset | Label of the optional banner action link | `Learn more` |
| `CSM_PORTAL_MAINTENANCE_BANNER_ACTION_URL` | string, unset | Target of the optional banner action link | `https://example.com/status` |
| `CSM_PORTAL_TOP_BANNERS` | array, `[]` | Ordered banners rendered above the header. Each item: `enabled`, `closeable`, `storageKey` (unique `localStorage` key remembering a dismissal; change it when the content changes), `html` (self-contained, sanitised: scripts and event handlers are stripped, inline styles kept), and optional `startsAt` / `expiresAt` (ISO 8601 timestamps with a UTC offset). See the template for full semantics | `[{ enabled: true, closeable: true, storageKey: "notice_2026_v1", html: "<div>...</div>" }]` |
| `CSM_PORTAL_TOP_BANNER_ENABLED` | boolean, `false` | Legacy single banner: gates `CSM_PORTAL_TOP_BANNER_HTML` when that is a string. Prefer `CSM_PORTAL_TOP_BANNERS` | `false` |
| `CSM_PORTAL_TOP_BANNER_HTML` | string or banner object, unset | Legacy single banner (string: non-closeable, gated by the key above; object: uses its own fields). Prefer `CSM_PORTAL_TOP_BANNERS` | `"<div>...</div>"` |
| `CSM_PORTAL_ANNOUNCEMENT_BANNER_VISIBLE` | boolean, `false` | Show the HTML announcement banner in the app layout | `false` |
| `CSM_PORTAL_ANNOUNCEMENT_BANNER_STORAGE_KEY` | string, `announcement_banner_v1` | `localStorage` key remembering that the announcement banner was dismissed | `announcement_2026_v1` |
| `CSM_PORTAL_ANNOUNCEMENT_BANNER_HTML` | string, empty | Announcement banner HTML (sanitised) | `"<div>...</div>"` |
| `CSM_PORTAL_MOBILE_APP_PROMPT_ENABLED` | boolean, `false` | Show a dismissible banner on mobile browsers pointing to a companion mobile app. Never blocks access. Suppressed when no store URL for the detected OS is set | `false` |
| `CSM_PORTAL_MOBILE_APP_IOS_STORE_URL` | string, unset | iOS store URL for the banner | `https://apps.example.com/ios/your-app` |
| `CSM_PORTAL_MOBILE_APP_ANDROID_STORE_URL` | string, unset | Android store URL for the banner | `https://apps.example.com/android/your-app` |
| `CSM_PORTAL_MOBILE_APP_INCLUDE_TABLETS` | boolean, `false` | Also show the banner on tablets | `false` |
| `CSM_PORTAL_ANNOUNCEMENT_TEST_PROJECT_KEY` | string, built-in default | Project key in which an announcement's "Send test" dry run creates its single test case. Override only if the environment needs a different sandbox project | `YOUR-TEST-PROJECT` |
| `CSM_MIGRATION_ONBOARDING_STATUS_ENABLED` | boolean or `"true"`/`"false"` string, **on** | Customer-onboarding status column on a project's Contacts tab (worst step status per contact, with step detail on hover). In the current code the column is on unless this is explicitly `false` / `"false"`; when off, no column is rendered and no request is made. The backend gates `GET /projects/{id}/onboarding-steps` under the same flag name, so both sides must agree | `false` |
| `CSM_PORTAL_DEV_BYPASS_ACCESS_CHECK` | boolean, `false` | **Local development only.** Skips the post-sign-in authorization checks (portal-role gate and per-section capability checks) for an account the backend has not provisioned. Authentication is unaffected. Set it only in your own git-ignored `config.js`, never in a shared or deployed one | `true` |

Boolean flags should be real booleans in `config.js`; `CSM_PORTAL_FEATURE_OVERRIDES` and `CSM_MIGRATION_ONBOARDING_STATUS_ENABLED` additionally tolerate string values for platforms that inject configuration as strings.

---

## Local full-stack mode (sandboxed dev)

```bash
pnpm run dev:local     # dev server acting as a local gateway
```

For environments that cannot reach a hosted identity provider or cloud
gateway (network-locked devcontainers, headless preview tooling). Sets
`CSM_PORTAL_LOCAL_BE=1`, which makes the Vite dev server (a) serve
`/config.js` with overrides (auth base URL pointed at a local mock OIDC
provider on `:9100`, backend base URL at the same-origin `/local-api`
path, sign-in/out redirects at the dev server's own origin) and (b) proxy
`/local-api/*` to the local Go backend on `:8080`, mapping the incoming
`Authorization: Bearer <jwt>` to `x-jwt-assertion` exactly as the production
gateway does. The app code runs unmodified: the real OIDC SDK performs a
genuine authorization-code + PKCE flow against the mock provider, which
signs real RS256 JWTs for any username you pick at its login form.

The overrides are appended to your `public/config.js`, so that file must
still exist (copy the template first). Applies only to the dev `serve`
command: `pnpm build` / `pnpm preview` can never carry this behaviour. The
mock provider, backend, entity service and seeded PostgreSQL it talks to must
be running separately (the local backend stack); this flag only changes the
dev server's behaviour. The `dev:local` script sets the variable with
POSIX shell syntax, so on Windows run it from Git Bash or WSL.
