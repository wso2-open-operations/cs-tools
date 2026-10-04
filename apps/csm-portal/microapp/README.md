# CSM Portal micro app

Mobile micro app for the CSM portal: the case, operations, customers and time-card flows of the
internal CS-engineer portal, laid out for a phone. It is a React 19 + Vite single-page app
(hash-routed) built as static assets.

## How it is hosted

The app is not opened directly by users. A React Native super app loads the built bundle in a
WebView and exposes a `window.nativebridge` object plus `window.ReactNativeWebView.postMessage`.
The bridge wrappers live in `src/components/microapp-bridge`; they are used for tokens, safe-area
insets, alerts, local data, opening URLs and forwarding logs to the host.

## How tokens arrive

The host performs sign-in. The micro app asks it for the access token and the ID token through
the bridge (`requestToken` / `requestIdToken`) and receives them through `resolveToken` /
`resolveIdToken`.

- Tokens are held **in memory only** (`src/services/auth.ts`); nothing is written to web storage.
- Every token refresh (launch, API requests, the case activity stream) shares one in-flight
  promise owned by `auth.ts`, and the bridge keeps one pending request per topic with a list of
  waiters, so overlapping callers never overwrite each other's resolver. A bridge request that
  is not answered within about 10 seconds is rejected.
- The API client (`src/services/apiClient.ts`) attaches the tokens to each request and retries
  once after a 401 with a forced refresh.

### Ending the session

The bridge has no sign-out message today. When the user signs out of the host, the host should
call:

```js
window.csmMicroApp.clearSession();
```

This drops both tokens, clears the signed-in user and empties the React Query cache.

## Running it

```sh
npm ci
cp .env.example .env   # set VITE_BACKEND_URL (and optionally VITE_STREAM_URL)
npm run dev            # local dev server
npm run build          # type-check and production build into dist/
npm run lint
```

Outside the super app `window.nativebridge` is absent, so token requests are rejected and API
calls will not authenticate; run it inside the host (or a harness that implements the bridge) to
exercise real flows.
