# Security Advisory Patches Portal — User & Developer Guide

This guide explains how the **Security Advisory Patches Portal** fits together: advisory **PDFs** live in an **Azure Files** share, a **Ballerina** backend streams bytes by path, and a minimal **React** app signs users in with **Asgardeo**. Any URL whose path **ends with `.pdf`** loads that file; otherwise you see **404**.

---

## 1. What the portal does

| Role | What you do |
|------|-------------|
| **Content publisher** | Upload or update PDFs (and folders) in the configured **Azure File Share** (Azure Portal, Storage Explorer, AzCopy, automation). |
| **End user (e.g. customer)** | Sign in with **Asgardeo** and open the **PDF link** you were sent (path ends with **`.pdf`**). |
| **Developer / operator** | Configure Azure credentials and share name, run or deploy the backend and webapp, tune CORS and `config.js` for each environment. |

The webapp does **not** upload files to Azure. Publishing is always done **outside** the portal.

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

- **Frontend**: Reads runtime settings from `public/config.js`. After sign-in, paths ending in **`.pdf`** map to `GET /file?path=…` (share-relative path); the PDF is shown in-page. Other paths show **404** unless you are on **`/`** (short help text).
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

The backend validates `path` with an allowlist-style pattern (alphanumeric, hyphen, underscore, dot, space, slash, URL-encoded segments). Prefer stable paths so emailed links stay valid.

### 4.4 When updates appear

The next successful fetch loads the **current** bytes from Azure (reload the page if you replaced the file).

---

## 5. Guide for developers

### 5.1 Repository layout

| Path | Purpose |
|------|---------|
| `backend/` | Ballerina package `security_advisories_fileshare` — health + `/file` |
| `webapp/` | React SPA — Asgardeo, Redux (auth only), PDF viewer |

### 5.2 Prerequisites

- **Ballerina** `2201.12.9` (`backend/Ballerina.toml`)
- **Node.js** compatible with `react-scripts` 5 / TypeScript 4.9

### 5.3 Backend configuration

Same `Config.toml` / `Config.toml.local` shape as before (`file_share` + credentials). Run:

```bash
cd backend && bal run
```

Listener **9090** by default; startup fails fast if the share is unreachable.

### 5.4 Backend HTTP API

| Method | Path | Query | Description |
|--------|------|--------|-------------|
| `GET` | `/health` | — | Liveness: file share reachable |
| `GET` | `/file` | `path` required | PDF (or other) bytes; `Content-Disposition: inline` |

The server runs **`url:decode`** on the `path` query value (`UTF-8`) so `%2F` becomes `/` when needed.

Invalid `path` → **400**; path valid but missing on the share (Azure **404**) → **404**; other download failures → **500**.

The `path` query must be the **share-relative** path Azure expects (folder names, spaces, casing). The SPA maps lowercase **kebab-case** directory segments in the URL to that shape before calling `/file`; you can also send the literal path with **`%20`** for spaces (e.g. `Security%20Patches/...`).

### 5.5 CORS

`backend/service.bal` allows `http://localhost:3000` and `http://127.0.0.1:3000` by default.

### 5.6 Webapp configuration (`config.js`)

The app loads `public/config.js` before the bundle. Define `window.config` with at least:

| Key | Purpose |
|-----|---------|
| `APP_NAME` | Title context |
| `ASGARDEO_BASE_URL` | Asgardeo server URL |
| `ASGARDEO_CLIENT_ID` | OIDC client ID |
| `AUTH_SIGN_IN_REDIRECT_URL` | Post-login redirect URI (typically site root `/`) |
| `AUTH_SIGN_OUT_REDIRECT_URL` | Post-logout redirect |
| `BACKEND_BASE_URL` | API origin for `/health` and `/file` |

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

Default dev server **3000** (matches backend CORS).

#### Using `patches.wso2.com` (or another hostname) on your machine

You do **not** need that hostname for routing—the app only cares about paths such as `/patches/…/*.pdf`. Asgardeo **does** compare redirect URIs exactly, so if production uses `https://patches.wso2.com/` but your dev Asgardeo app only lists `http://localhost:3000/`, use one of these:

1. **Add localhost redirects** in the Asgardeo SPA app (`http://localhost:3000/`, `http://127.0.0.1:3000/`) and keep testing at `http://localhost:3000`—simplest.

2. **Match production hostname locally**:

   - Map the name to your loopback interface (macOS/Linux: edit `/etc/hosts` as admin):

     ```text
     127.0.0.1 patches.wso2.com
     ```

   - Start the webapp bound to that host so the browser URL matches what you register in Asgardeo:

     ```bash
     npm run start:patches-host
     ```

     Then open **`http://patches.wso2.com:3000/`** (not `https` unless you terminate TLS locally).

   - Set `AUTH_SIGN_IN_REDIRECT_URL` and `AUTH_SIGN_OUT_REDIRECT_URL` in `public/config.js` to **`http://patches.wso2.com:3000/`** (same origin + port + trailing slash as in Asgardeo).

   - In Asgardeo, add **`http://patches.wso2.com:3000`** as an allowed redirect URL.

The backend [`backend/service.bal`](backend/service.bal) CORS list includes `http://patches.wso2.com:3000` for this pattern; add more origins there if you use another hostname.

### 5.7 Production authentication note

The SPA sends a Bearer token; the **backend does not validate JWT** in this codebase—place the API behind a private network, gateway, or similar.

### 5.8 Troubleshooting

| Symptom | Things to check |
|--------|------------------|
| Backend won’t start | Share name, credentials, firewall |
| 400 on `/file` | Path characters outside allowed pattern |
| Blank PDF | Wrong path mapping vs Azure layout; browser blocking blob iframe |
| Auth loops | Redirect URIs match `config.js` |
| Deep link → home after login | Keep sign-in redirect at site root; the app restores **`.pdf`** links from session |

---

## 6. Summary

- **Publishers** put PDFs in **Azure Files** and distribute links whose path ends with **`.pdf`**.  
- **Users** sign in with **Asgardeo** and open PDF links (path ends with **`.pdf`**). Invalid or missing files show **404**.  
- **Developers** run **`bal`** + **`npm`**, configure **`config.js`**, CORS, and network boundaries.

Code paths: `backend/service.bal`, `backend/modules/file_storage/`, `webapp/src/view/PatchesPdf/PatchesPdfPage.tsx`, `webapp/src/view/NotFound/NotFoundPage.tsx`, `webapp/src/app/AppHandler.tsx`.
