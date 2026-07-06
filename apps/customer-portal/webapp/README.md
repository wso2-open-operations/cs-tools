# Customer Portal Webapp

The Customer Portal Webapp is a React single-page application for customer success operations—project visibility, support cases, deployments, security updates, and related workflows. It authenticates users through Asgardeo (or WSO2 Identity Server) and talks to the [Customer Portal backend](../backend/README.md) over REST.

## Tech Stack

| Area | Technology |
|------|------------|
| UI | [React 19](https://react.dev/), [TypeScript](https://www.typescriptlang.org/), [Vite 7](https://vite.dev/) |
| Design system | [Oxygen UI](https://github.com/wso2/oxygen-ui) |
| Data fetching | [TanStack Query](https://tanstack.com/query/latest) |
| Routing | [React Router 7](https://reactrouter.com/) |
| Authentication | [@asgardeo/react](https://github.com/asgardeo/asgardeo-auth-react-sdk) |
| Rich text | [Lexical](https://lexical.dev/) |
| Unit tests | [Vitest](https://vitest.dev/), [React Testing Library](https://testing-library.com/docs/react-testing-library/intro/) |

## Prerequisites

- [Node.js](https://nodejs.org/) 20+ (LTS recommended)
- [pnpm](https://pnpm.io/) 9+
- A running Customer Portal [backend](../backend/README.md) instance (local or remote)
- An Asgardeo organisation (or WSO2 Identity Server tenant) with a registered SPA application

## Getting Started

All commands below are run from `apps/customer-portal/webapp`.

### 1. Install dependencies

```bash
pnpm install
```

### 2. Create runtime configuration

Configuration is **not** loaded from `.env` files at build time. Instead, the browser loads a runtime `config.js` before the React bundle starts (see `index.html`).

1. Copy the template:

   ```bash
   cp public/config.js.example public/config.js
   ```

2. Edit `public/config.js` and set the required values for your environment (auth URLs, client ID, backend base URL, and so on).

3. **Do not commit `public/config.js`** — it is listed in `.gitignore` and may contain environment-specific URLs. Use `public/config.js.example` as the shared reference.

4. Restart the dev server (or redeploy) after changing `config.js`; values are read once at page load.

### 3. Configure Asgardeo / Identity Server

Register a **Single Page Application** in your identity provider and align these settings with `public/config.js`:

| IdP setting | Must match |
|-------------|------------|
| Allowed callback URLs | `CUSTOMER_PORTAL_AUTH_SIGN_IN_REDIRECT_URL` |
| Allowed logout URLs | `CUSTOMER_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL` |

For local development, use `http://localhost:3000` for both redirect URLs unless you change the Vite dev server port.

### 4. Start the backend (local)

If you are running the stack locally, start the Ballerina backend first:

```bash
# from apps/customer-portal/backend
bal run
```

The backend listens on `http://localhost:9090` by default. Point `CUSTOMER_PORTAL_BACKEND_BASE_URL` in `config.js` at your backend base path (for example `http://localhost:9090` or the Choreo gateway URL including the API version segment). See the [backend README](../backend/README.md) for `config.toml.local` and integration setup.

### 5. Run the webapp

```bash
pnpm run dev
```

Open [http://localhost:3000](http://localhost:3000). The dev server uses port **3000** (configured in `vite.config.ts`).

## Available Scripts

| Command | Description |
|---------|-------------|
| `pnpm run dev` | Start Vite dev server on port 3000 |
| `pnpm run build` | Type-check (`tsc -b`) and production build to `dist/` |
| `pnpm run preview` | Serve the production build locally |
| `pnpm run lint` | Run ESLint |
| `pnpm run test` | Run Vitest unit tests (watch mode) |

## Configuration

### How runtime config works

```text
index.html
  └── <script src="/config.js">     ← sets window.config
        └── React app (src/config/*) ← reads window.config at startup
```

- **Template:** `public/config.js.example` — fully commented reference for every key.
- **Local / deploy file:** `public/config.js` — your environment-specific values (gitignored).
- **Type definition:** `src/config/portalConfig.ts` — `CustomerPortalWindowConfig` interface.

Optional keys can be omitted; the app applies safe defaults in `src/config/*` modules.

### Sample `config.js` structure

Create `public/config.js` with a single `window.config` object. Minimal local development example:

```javascript
window.config = {
  // Required — authentication
  CUSTOMER_PORTAL_AUTH_BASE_URL: "https://api.asgardeo.io/t/<your-tenant>",
  CUSTOMER_PORTAL_AUTH_CLIENT_ID: "<your-client-id>",
  CUSTOMER_PORTAL_AUTH_SIGN_IN_REDIRECT_URL: "http://localhost:3000",
  CUSTOMER_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL: "http://localhost:3000",

  // Required — backend Url
  CUSTOMER_PORTAL_BACKEND_BASE_URL: "http://localhost:9090",

  // Optional — UI and logging
  CUSTOMER_PORTAL_THEME: "acrylicOrange",
  CUSTOMER_PORTAL_LOG_LEVEL: "INFO",
  CUSTOMER_PORTAL_CHATBOT_WEBSOCKET_URL: "wss://<host>/customer-portal-backend/websocket/v1.0/ws",
  CUSTOMER_PORTAL_FLOATING_NOVERA_ENABLED: false,

  // Optional — maintenance banner (below header)
  CUSTOMER_PORTAL_MAINTENANCE_BANNER_VISIBLE: false,
  // CUSTOMER_PORTAL_MAINTENANCE_BANNER_SEVERITY: "warning",
  // CUSTOMER_PORTAL_MAINTENANCE_BANNER_TITLE: "Scheduled maintenance",
  // CUSTOMER_PORTAL_MAINTENANCE_BANNER_MESSAGE: "Some services may be unavailable.",
  // CUSTOMER_PORTAL_MAINTENANCE_BANNER_ACTION_LABEL: "Learn more",
  // CUSTOMER_PORTAL_MAINTENANCE_BANNER_ACTION_URL: "https://status.example.com",

  // Optional — top-of-page HTML banners (inline styles only)
  CUSTOMER_PORTAL_TOP_BANNERS: [
    {
      enabled: false,
      closeable: false,
      storageKey: "event_banner_2026_v1",
      html: '<div style="background:#000;height:3rem"><a href="https://example.com" target="_blank"><img style="object-fit:cover;height:100%;width:100%" src="https://cdn.example.com/banner.png" alt=""></a></div>',
    },
    {
      enabled: false,
      closeable: true,
      storageKey: "promo_banner_2026_v1",
      html: '<div style="background:#000;height:3rem"><img style="object-fit:cover;height:100%;width:100%" src="https://cdn.example.com/promo.png" alt=""></div>',
    },
  ],

  // Optional — dismissable announcement banner (include data-close-banner on close control)
  CUSTOMER_PORTAL_ANNOUNCEMENT_BANNER_VISIBLE: false,
  // CUSTOMER_PORTAL_ANNOUNCEMENT_BANNER_STORAGE_KEY: "announcement_banner_2026_v1",
  // CUSTOMER_PORTAL_ANNOUNCEMENT_BANNER_HTML: '<div style="position:relative;background:#1f2937;padding:12px 48px 12px 16px;color:#f9fafb"><span>Portal upgrade this weekend.</span><button data-close-banner="true" style="position:absolute;top:50%;right:12px;transform:translateY(-50%)">&times;</button></div>',

  // Optional — mobile: redirect phones to WSO2 Super App stores
  CUSTOMER_PORTAL_MOBILE_APP_PROMPT_ENABLED: false,
  CUSTOMER_PORTAL_MOBILE_APP_IOS_STORE_URL: "https://apps.apple.com/app/wso2/id<ios-app-id>",
  CUSTOMER_PORTAL_MOBILE_APP_ANDROID_STORE_URL:
    "https://play.google.com/store/apps/details?id=com.wso2.superapp",
  CUSTOMER_PORTAL_MOBILE_APP_INCLUDE_TABLETS: false,
};
```

Copy from the template for the full commented version:

```bash
cp public/config.js.example public/config.js
```

### Required settings

These must be set in `public/config.js` or the app throws at startup:

| Key | Description |
|-----|-------------|
| `CUSTOMER_PORTAL_AUTH_BASE_URL` | Asgardeo or IS tenant base URL (e.g. `https://api.asgardeo.io/t/<org>`) |
| `CUSTOMER_PORTAL_AUTH_CLIENT_ID` | OAuth 2.0 SPA client ID |
| `CUSTOMER_PORTAL_AUTH_SIGN_IN_REDIRECT_URL` | Post-login redirect URL (must match IdP callback allowlist) |
| `CUSTOMER_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL` | Post-logout redirect URL (must match IdP logout allowlist) |
| `CUSTOMER_PORTAL_BACKEND_BASE_URL` | Customer Portal REST API base URL (no trailing slash) |

### Optional settings

| Key | Default | Description |
|-----|---------|-------------|
| `CUSTOMER_PORTAL_THEME` | `acrylicOrange` | Oxygen UI theme: `acrylicOrange`, `acrylicPurple`, `highContrast`, `classic` |
| `CUSTOMER_PORTAL_LOG_LEVEL` | `ERROR` | Console log level: `ERROR`, `WARN`, `INFO`, `DEBUG` |
| `CUSTOMER_PORTAL_CHATBOT_WEBSOCKET_URL` | — | WebSocket URL for the AI chatbot; omit to disable |
| `CUSTOMER_PORTAL_FLOATING_NOVERA_ENABLED` | `false` | Show floating Novera assistant button on all pages |

#### Maintenance banner

Site-wide alert below the header (see `src/config/notificationBannerConfig.ts`).

| Key | Default | Description |
|-----|---------|-------------|
| `CUSTOMER_PORTAL_MAINTENANCE_BANNER_VISIBLE` | `false` | Show maintenance banner |
| `CUSTOMER_PORTAL_MAINTENANCE_BANNER_SEVERITY` | `info` | `info`, `warning`, `error`, `success` |
| `CUSTOMER_PORTAL_MAINTENANCE_BANNER_TITLE` | `""` | Banner heading |
| `CUSTOMER_PORTAL_MAINTENANCE_BANNER_MESSAGE` | `""` | Banner body text |
| `CUSTOMER_PORTAL_MAINTENANCE_BANNER_ACTION_LABEL` | — | Optional CTA label |
| `CUSTOMER_PORTAL_MAINTENANCE_BANNER_ACTION_URL` | — | Optional CTA URL |

#### Top banners

Configurable HTML strips above the main header (`CUSTOMER_PORTAL_TOP_BANNERS` array). Each item supports:

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | `boolean` | Render this banner |
| `closeable` | `boolean` | Show dismiss control; persists dismissal in `localStorage` |
| `storageKey` | `string` | Unique key for dismissal state (change when content updates) |
| `html` | `string` | Inline-styled HTML (host images on a stable CDN) |

#### Announcement banner

Dismissable HTML banner below top banners (`src/config/announcementBannerConfig.ts`).

| Key | Default | Description |
|-----|---------|-------------|
| `CUSTOMER_PORTAL_ANNOUNCEMENT_BANNER_VISIBLE` | `false` | Show announcement banner |
| `CUSTOMER_PORTAL_ANNOUNCEMENT_BANNER_STORAGE_KEY` | `announcement_banner_v1` | `localStorage` key for dismissal |
| `CUSTOMER_PORTAL_ANNOUNCEMENT_BANNER_HTML` | `""` | Full HTML; use `data-close-banner="true"` on a control to dismiss |

#### Mobile app download prompt

On supported mobile devices, block the web UI and prompt users to install the WSO2 Super App (`src/config/mobileAppConfig.ts`).

| Key | Default | Description |
|-----|---------|-------------|
| `CUSTOMER_PORTAL_MOBILE_APP_PROMPT_ENABLED` | `false` | Enable mobile download gate |
| `CUSTOMER_PORTAL_MOBILE_APP_IOS_STORE_URL` | — | Apple App Store URL |
| `CUSTOMER_PORTAL_MOBILE_APP_ANDROID_STORE_URL` | — | Google Play Store URL |
| `CUSTOMER_PORTAL_MOBILE_APP_INCLUDE_TABLETS` | `false` | Also prompt on iPad / Android tablets |

For field-by-field examples and sample HTML, see **`public/config.js.example`**.

### Deploying configuration

For production:

1. Build the app: `pnpm run build`.
2. Ship the `dist/` output **and** a environment-specific `config.js` placed at the root of the served site (same path as in dev: `/config.js`).
3. Mount or inject `config.js` via your platform (Kubernetes ConfigMap, CDN, reverse proxy, Choreo config mount, etc.) without rebuilding the bundle when URLs change.

Because config is runtime-only, you can promote the same `dist/` artifact across environments by swapping only `config.js`.

## Project Structure

```text
webapp/
├── public/
│   ├── config.js.example    # Documented config template (commit this)
│   └── config.js            # Your local/deploy config (gitignored)
├── src/
│   ├── api/                 # Shared API hooks
│   ├── components/          # Shared UI components
│   ├── config/              # Reads window.config (auth, API, theme, banners, …)
│   ├── features/            # Feature modules (see below)
│   ├── hooks/               # Shared hooks
│   ├── layouts/             # App shells, guards
│   ├── providers/           # Cross-cutting providers (e.g. mobile gate)
│   └── main.tsx             # Application entry
├── index.html               # Loads /config.js before the bundle
└── vite.config.ts           # Aliases, dev server port, Vitest merge
```

### Feature modules

Domain code lives under `src/features/`:

| Module | Purpose |
|--------|---------|
| `announcements` | Announcement banner behaviour |
| `dashboard` | Project dashboard and stats |
| `engagements` | Customer engagements |
| `operations` | Change requests, service requests |
| `project-details` | Single-project views, deployments, time tracking |
| `project-hub` | Project listing and navigation |
| `security` | Product vulnerabilities |
| `settings` | Project settings, contacts, registry tokens |
| `support` | Cases, conversations, Novera AI assistant |
| `updates` | Product update levels |
| `usage-metrics` | Usage and time-card metrics |

Pages compose components from `src/components/` and their feature folder; avoid defining inline page-level components in route files.

## Import Aliases

Path aliases are defined in `vite.config.ts` and `tsconfig.app.json`. Prefer these over deep relative imports:

| Alias | Resolves to |
|-------|-------------|
| `@/` | `src/` |
| `@api/` | `src/api/` |
| `@assets/` | `src/assets/` |
| `@components/` | `src/components/` |
| `@config/` | `src/config/` |
| `@constants/` | `src/constants/` |
| `@context/` | `src/context/` |
| `@features/` | `src/features/` |
| `@hooks/` | `src/hooks/` |
| `@layouts/` | `src/layouts/` |
| `@providers/` | `src/providers/` |
| `@utils/` | `src/utils/` |

Feature-specific shortcuts (e.g. `@case-details`, `@deployments`, `@time-tracking`) are also available—see `vite.config.ts` for the full list.

Example:

```typescript
import { authConfig } from "@config/authConfig";
import { useGetProjects } from "@api/useGetProjects";
```

## Logging

A small logger respects `CUSTOMER_PORTAL_LOG_LEVEL` from `window.config`. Use the `useLogger` hook in components:

```typescript
import { useLogger } from "@hooks/logger";

const logger = useLogger();
logger.info("Loaded project list");
```

Configuration is resolved in `src/config/loggerConfig.ts`.

## Testing

Vitest runs in watch mode by default:

```bash
pnpm run test
```

Tests mock `window.config` where needed and use setup in `src/vitest.setup.ts` (Asgardeo and API client mocks).

## Troubleshooting

| Symptom | Likely cause |
|---------|----------------|
| Blank page / immediate error in console | Missing or invalid `public/config.js`; copy from `config.js.example` |
| `Auth Config Error: Missing required configuration` | Auth keys empty or typo in `config.js` |
| `Missing required configuration: CUSTOMER_PORTAL_BACKEND_BASE_URL` | Backend URL not set |
| Login redirect mismatch | Callback/logout URLs in `config.js` do not match IdP app settings |
| API 401 / empty data locally | Backend not running, wrong `BACKEND_BASE_URL`, or token not forwarded—see [backend auth docs](../backend/README.md#authentication) |
| Config change not applied | Restart dev server or hard-refresh; `config.js` is cached at first load |
| Chatbot unavailable | `CUSTOMER_PORTAL_CHATBOT_WEBSOCKET_URL` unset or backend WebSocket not configured |

