# Security Advisory Patches Portal — User & Developer Guide

This guide explains how the **Security Advisory Patches Portal** fits together: advisory **PDFs** live in an **Azure Files** share, a **Ballerina** backend streams bytes by path, and a minimal **React** app signs users in with **Asgardeo**. Any URL whose path **ends with `.pdf`** loads that file; otherwise you see **404**.

---

## 1. What the portal does

| Role | What you do |
|------|-------------|
| **Content publisher** | Upload or update PDFs (and folders) in the configured **Azure File Share** (Azure Portal, Storage Explorer, AzCopy, automation). |
| **End user (e.g. customer)** | Sign in with **Asgardeo** and open the **PDF link** you were sent (path ends with **`.pdf`**). |
| **Developer / operator** | Configure Azure credentials and share name, run or deploy the backend and webapp, tune `config.js`. |

The webapp does **not** upload files to Azure. Publishing is always done **outside** via Azure Portal.

---

## 2. Architecture (high level)

```text
┌─────────────────┐     HTTPS + Bearer token      ┌──────────────────┐
│  React webapp   │ ─────────────────────────────► │  Ballerina API   │
│  (Asgardeo OIDC)│ ◄───────────────────────────── │  (port 9090)     │
└────────┬────────┘         PDF blob               └────────┬─────────┘
         │                                                    │
         │  window.config (config.js)                         │  account key or SAS
         │  BACKEND_BASE_URL, Asgardeo URLs                   ▼
         │                                          ┌──────────────────┐
         └────────────────────────────────────────► │  Azure File Share│
                                                    └──────────────────┘
```

- **Frontend**: Reads runtime settings from `public/config.js`. After sign-in, paths ending in **`.pdf`** map to **`GET /files/{id}`** where **`id`** is the share-relative path as **one** URL-encoded segment (`encodeURIComponent`); the PDF is shown in-page. Other paths show **404** unless you are on **`/`** (short help text).
- **Backend**: Uses `ballerinax/azure_storage_service.files` to **download file bytes only**. It performs a **health check** against the file share at startup.

---

## 3. Guide for end users

### 3.1 Sign-in and opening the site

1. Use the portal hostname your organization gave you (for example `https://patches.example.com/`).
2. You are redirected to **Asgardeo** to sign in.
3. If you land on **/** after login without having opened a PDF link first, you see a short message: open the **full URL** you received whose path ends with **`.pdf`**, or sign out and open the link from your email again.
4. If you started from such a link before signing in, that URL is restored after login when the identity provider sends you back to the site root.

### 3.2 PDF viewing only

- There is **no** folder browser—only the PDF viewer when the link is valid and the file exists.
- **Hyphen encoding** (legacy links): doubled `--` for literal hyphens and single `-` for spaces in a segment; segments without `--` keep `-` as-is. Prefer **`%20`** for spaces when building links.
- **Folder slugs vs Azure**: Path segments for **directories** that look like lowercase kebab-case (e.g. `security-patches`, `january`) are turned into Azure-style names (`Security Patches`, `January`) before calling the API. The **PDF file name** segment is never changed—use the exact name in the share (e.g. `WSO2-2025-3857_CVE-2025-0326.pdf`). You can also put real folder names in the URL with **`%20`** for spaces; those segments are left as-is.
- If the first URL segment is **`patches`**, it is stripped when resolving the Azure path (existing **`/patches/…`** links keep working).

### 3.3 Invalid links and missing files

- If the path does not end with **`.pdf`**, you get a **404** page (except **`/`**, which shows the help text).
- If the path ends with **`.pdf`** but the backend cannot return the file, you also see **404**.

### 3.4 Sign out

Use **Sign out** in the header to leave the app (behavior depends on your IdP).

---

## 4. Guide for content publishers (Azure File Share)

### 4.1 Where files go

PDFs are served from **one** Azure file share (storage account name, share name, access key or SAS in Ballerina config).

### 4.2 Upload or update content

Use Portal, Storage Explorer, AzCopy, or automation—same as any Azure Files workflow.

### 4.3 Naming and path rules

The backend validates the decoded share path (from **`id`**) using the same segment rules as the file-storage module (no `..`, no control characters, sane length) before calling Azure.

### 4.4 When updates appear

The next successful fetch loads the **current** bytes from Azure (reload the page if you replaced the file).

---

## 5. Guide for developers

### 5.1 Repository layout

| Path | Purpose |
|------|---------|
| `backend/` | Ballerina package `security_advisories_fileshare` — `modules/authorization` (JWT / roles), `modules/file_storage`, `GET /health`, `GET /files/[id]` |
| `webapp/` | React SPA — Asgardeo, Redux (auth only), PDF viewer |

### 5.2 Prerequisites

- **Ballerina** `2201.12.9` (`backend/Ballerina.toml`)
- **Node.js** compatible with `react-scripts` 5 / TypeScript 4.9

### 5.3 Backend configuration

`Config.toml` / `Config.toml.local` follow the same layout as [`webapps/backend-template`](../../webapps/backend-template): **Azure file share** settings plus **authorization** (Asgardeo group → API access).

**Authorization** (required for `GET /files/[id]`; see `backend/modules/authorization/`):

```toml
[security_advisories_fileshare.authorization.authorizedRoles]
securityPatchesUserRole = "<Asgardeo-group-name>"
```

- Set `securityPatchesUserRole` to the **exact** string that appears in the ID token **`groups`** array.
- For **`/files/...`**, the SPA sends the Asgardeo **ID token** on **`x-jwt-assertion`**. OIDC scopes are set in `webapp/src/config/config.ts` to **`openid`**, **`email`**, and **`groups`** so the ID token includes **`email`** and **`groups`** for `CustomJwtPayload`.
- **`GET /health`** does **not** require a JWT (liveness / probes). **`OPTIONS`** preflight is also allowed without JWT.

**File share** (same `security_advisories_fileshare.file_storage` tables as before). Then run:

```bash
cd backend && bal run
```

Listener **9090** by default; startup fails fast if the share is unreachable.

### 5.4 Backend HTTP API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Liveness: file share reachable (**no** `x-jwt-assertion`) |
| `GET` | `/files/[id]` | **`id`**: single path segment — percent-encode the whole share-relative path (e.g. `encodeURIComponent("Security Patches/x.pdf")`). PDF bytes; requires **`x-jwt-assertion`** (ID token) and **`securityPatchesUserRole`**; `Content-Disposition: inline` |

The server runs **`url:decode`** on **`id`** (`UTF-8`) after routing so nested `%` sequences resolve to the share-relative path.

Missing/invalid JWT or wrong groups → **403** / **500** as returned by the JWT interceptor; missing user context after auth → **400**. Invalid path after decode → **400**; path valid but missing on the share (Azure **404**) → **404**; other download failures → **500**.

**`id`** must represent the **share-relative** path Azure expects (folder names, spaces, casing). The SPA maps lowercase **kebab-case** directory segments in the browser URL to that shape, then encodes the full path for **`/files/{id}`**; you can also call the API with the literal path encoded (e.g. `Security%20Patches%2F...`).

### 5.5 Webapp configuration (`config.js`)

The app loads `public/config.js` before the bundle. Define `window.config` with at least:

| Key | Purpose |
|-----|---------|
| `APP_NAME` | Title context |
| `ASGARDEO_BASE_URL` | Asgardeo server URL |
| `ASGARDEO_CLIENT_ID` | Asgardeo client ID |
| `AUTH_SIGN_IN_REDIRECT_URL` | Post-login redirect URI (typically site root `/`) |
| `AUTH_SIGN_OUT_REDIRECT_URL` | Post-logout redirect |
| `BACKEND_BASE_URL` | API origin for `/health` and `/files` |

Example:

```javascript
window.config = {
  APP_NAME: 'Security Advisory Patches Portal',
  ASGARDEO_BASE_URL: 'https://api.asgardeo.io/t/<org>',
  ASGARDEO_CLIENT_ID: '<client-id>',
  AUTH_SIGN_IN_REDIRECT_URL: 'http://localhost:3000/',
  AUTH_SIGN_OUT_REDIRECT_URL: 'http://localhost:3000/',
  BACKEND_BASE_URL: 'http://localhost:9090',
};
```

From `webapp/`:

```bash
npm install
npm start
```

Default dev server **`http://localhost:3000`**. Register that origin in your Asgardeo SPA app (redirect URLs and allowed origins as required), for example `http://localhost:3000/` with a trailing slash so it matches `AUTH_SIGN_IN_REDIRECT_URL` / `AUTH_SIGN_OUT_REDIRECT_URL` in `public/config.js`. You can also add **`http://127.0.0.1:3000/`** if you open the app via `127.0.0.1`. This repo assumes local dev uses **localhost only**, not a custom hostname in `/etc/hosts`.

### 5.6 Production authentication note

The backend reads **`x-jwt-assertion`** (JWT decode) and checks Asgardeo **`groups`** against `authorizedRoles` (see `backend/modules/authorization/authorization.bal`).

### 5.7 Troubleshooting

| Symptom | Things to check |
|--------|------------------|
| Backend won’t start | Share name, credentials, firewall |
| 400 on `/files/...` | Path fails segment validation (e.g. `..`, empty segment); or missing user context after auth; or bad **`id`** encoding |
| 403 on `/files/...` | User not in `securityPatchesUserRole` group; ID token missing `groups` |
| 400 "User information header not found!" | Request reached `/files/...` without expected auth context; verify JWT interceptor wiring and that SPA sends `x-jwt-assertion` (see `apiService.ts`) |
| 500 "Missing invoker info header" | SPA not sending `x-jwt-assertion` (see `apiService.ts`) |
| 500 "Malformed Invoker info object!" | Decode the ID token: it must include **`email`** and **`groups`**. Use scopes **`openid`**, **`email`**, **`groups`** in `webapp/src/config/config.ts`; in Asgardeo enable those attributes on the app and assign the user to a group matching `securityPatchesUserRole` |
| Blank PDF | Wrong path mapping vs Azure layout; browser blocking blob iframe |
| Auth loops | Redirect URIs match `config.js` |
| Deep link → home after login | Keep sign-in redirect at site root; the app restores **`.pdf`** links from session |

---

## 6. Summary

- **Publishers** put PDFs in **Azure Files** and distribute links whose path ends with **`.pdf`**.  
- **Users** sign in with **Asgardeo** and open PDF links (path ends with **`.pdf`**). Invalid or missing files show **404**.  
- **Developers** run **`bal`** + **`npm`**, configure **`config.js`**, gateway CORS, and network boundaries.

Code paths: `backend/service.bal`, `backend/modules/file_storage/`, `webapp/src/view/PatchesPdf/PatchesPdfPage.tsx`, `webapp/src/view/NotFound/NotFoundPage.tsx`, `webapp/src/app/AppHandler.tsx`.
