# WSO2 Customer Portal — Microapp

A React application embedded inside a WSO2 super-app as a WebView-hosted microapp. It gives enterprise customers a self-service portal to manage support cases, change requests, service requests, engagements, announcements, and AI-assisted chat via Novera.

---

## Architecture Overview

This is not a standalone web app. It runs inside a **React Native WebView**, and several architectural decisions flow from that constraint.

**Authentication is delegated to the host.** The app never handles OAuth flows. Instead it requests tokens on demand via a JavaScript bridge (`window.nativebridge`) that the host injects before the page loads. Tokens are cached in `localStorage` and refreshed before every API call.

**Routing uses HashRouter.** All routes are expressed as `/#/path`, which works safely inside a WebView where there is no HTTP server to handle path-based navigation. The Vite build uses `base: "./"` so all asset URLs in the built `index.html` are relative — the zip can be served from any path or loaded as `file://` without reconfiguration.

**Layout is declared per-page, not configured per-route.** Each page calls `useDeclareLayout()` from within its component body to set the AppBar title, back button, exit button, tab index, and slot content. A `LayoutProvider` keyed to the current pathname resets this state on every route transition, so stale layout from one page never bleeds into the next.

**Feature access is runtime-gated.** Tabs (Service Requests, Change Requests, SRA, Engagements) and the Novera chat flow are enabled or disabled based on a `ProjectFeaturesDto` fetched per project. The app reads these flags from `ProjectContext` throughout.

**Server state is managed with TanStack React Query.** All API calls go through an axios client that injects auth headers and handles 401 retries. Pagination uses a record-offset strategy (`offset + limit`) across all list queries.

**Novera AI uses a raw WebSocket.** The WebSocket URL is derived from `VITE_BACKEND_URL`. Auth tokens are passed as subprotocols (the standard browser workaround since WebSocket handshakes can't carry custom headers). Streaming responses arrive as a typed event sequence (`thinking_start`, `token`, `final`) consumed by a state machine in `useStream` that drives the animated typing UI.

---

## Folder Structure

```
src/
  app/              Router setup and top-level provider composition
  bridge/           All communication with the native host (tokens, storage, alerts, URL opening)
  config/           API endpoint constants and app-wide config
  context/          React contexts — project, user, layout, filters, snackbar, preview, theme
  features/         Feature modules, each owning its API, hooks, components, types and mappers
  infrastructure/   axios client with auth interceptor, native-bridged logger
  pages/            One page component per route
  shared/           Cross-feature components, hooks, constants, types and utilities
  theme/            MUI theme (typography overrides on top of WSO2 Oxygen UI)
```

Each feature module under `features/` is self-contained:

```
features/<feature>/
  api/          API functions and React Query option factories
  components/   UI components
  hooks/        Business logic hooks
  types/        DTOs (API shape) and models (domain shape)
  mappers/      DTO → model transformations
```

---

## Getting Started

**Install dependencies**

```bash
npm install
```

**Configure environment** — create a `.env` file:

```
VITE_BACKEND_URL=https://your-backend.example.com/v1.0
```

This is the only required variable. It sets the API base URL and is used to derive the Novera WebSocket endpoint.

**Run the dev server**

```bash
npm run dev        # http://localhost:3000
npm run build      # type-check + build to dist/
npm run lint:fix   # ESLint auto-fix
npm run format     # Prettier
```

---

## Building as a Microapp

```bash
npm run build
cd dist && zip -r ../customer-portal-microapp.zip . && cd ..
```

The host loads `index.html` in a WebView and must inject `window.nativebridge` before the page loads, exposing resolver methods for token delivery, secure storage, safe-area insets, confirm dialogs, and URL opening. Sending `"close_webview"` via `ReactNativeWebView.postMessage` dismisses the WebView and returns the user to the app picker.
