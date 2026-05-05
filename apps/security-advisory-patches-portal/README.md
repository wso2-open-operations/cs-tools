# Security Advisory Patches Portal — User & Developer Guide

This guide explains how the **Security Advisory Patches Portal** fits together: an **Azure Files** file share holds advisory documents, a **Ballerina** backend lists and downloads them, and a **React** web application lets authenticated users browse, preview, and download files.

---

## 1. What the portal does

| Role | What you do |
|------|-------------|
| **Content publisher** | Upload or update files and folders in the configured **Azure File Share** (Azure Portal, Azure Storage Explorer, AzCopy, automation, etc.). |
| **End user (e.g. customer)** | Sign in with **Asgardeo**, open folders, preview supported file types, download files, and share deep links to a specific file. |
| **Developer / operator** | Configure Azure credentials and share name, run or deploy the backend and webapp, tune CORS and `config.js` for each environment. |

The webapp does **not** upload files to Azure. Publishing is always done **outside** the portal, directly against the file share the backend is wired to.

---

## 2. Architecture (high level)

```text
┌─────────────────┐     HTTPS + Bearer token      ┌──────────────────┐
│  React webapp   │ ─────────────────────────────► │  Ballerina API   │
│  (Asgardeo OIDC)│ ◄───────────────────────────── │  (port 9090)     │
└────────┬────────┘         JSON / blobs           └────────┬─────────┘
         │                                                    │
         │  window.config (config.js)                         │  account key or SAS
         │  BACKEND_BASE_URL, Asgardeo URLs                   ▼
         │                                          ┌──────────────────┐
         └────────────────────────────────────────► │  Azure File Share│
                                                    └──────────────────┘
```

- **Frontend**: Reads runtime settings from `public/config.js` (`window.config`). After sign-in, API calls include an `Authorization: Bearer <ID token>` header (see `webapp/src/utils/apiService.ts`).
- **Backend**: Uses `ballerinax/azure_storage_service.files` to list directories/files and download file bytes. It performs a **health check** against the file share at startup.

---

## 3. Guide for end users

### 3.1 Sign-in

1. Open the portal URL provided by your organization.
2. You are redirected to **Asgardeo** to sign in.
3. After a successful login, you land on the file explorer (root of the share as exposed by the app).

If you opened a **direct link** to a folder or file before logging in, the app stores that path and sends you there after authentication.

### 3.2 Browse folders and files

- The left panel lists **folders** and **files** in the current directory.
- Click a **folder** to go deeper. Use **breadcrumbs** in the header to go up or jump to a parent folder.
- The root view is labeled **“Security Advisories”** in the UI when you are at the top level.

### 3.3 Open, preview, and download

- Click a **file** to select it. A **preview panel** opens on the right when a file is selected (you can drag the divider to resize).
- **In-browser preview** is available for:
  - **PDF** (`pdf`)
  - **Images**: `png`, `jpg`, `jpeg`, `gif`, `webp`, `svg`
- Other types (for example Office documents, archives) show a **“Preview not available”** message; use **Download** to open them locally.

### 3.4 Shareable URLs

- URLs are built with a `/patches/...` prefix and URL-encoded path segments (spaces and special characters are encoded).
- A link that ends with a **file name** (including an extension) opens that folder and selects the file for preview.
- You can copy the browser address bar and share it with others who have portal access (they must be able to sign in).

### 3.5 Sign out

Use **Sign out** in the header to leave the app and clear the Asgardeo session (behavior depends on your IdP configuration).

---

## 4. Guide for content publishers (Azure File Share)

### 4.1 Where files go

Everything listed in the portal comes from **one** Azure file share, configured for the backend as:

- **Storage account** name  
- **File share** name  
- **Authorization**: storage account **access key** or **SAS**, as configured (`authorizationMethod`: `accessKey` or SAS as supported by the Azure Files connector)

Treat the storage account key and SAS as **secrets**. Store them in secure configuration or a secret manager, not in source control.

### 4.2 How to upload or update content

Use any standard method for Azure Files, for example:

- [Azure Portal](https://portal.azure.com) → Storage account → File shares  
- [Azure Storage Explorer](https://azure.microsoft.com/products/storage/storage-explorer/)  
- [AzCopy](https://learn.microsoft.com/azure/storage/common/storage-use-azcopy-v10) or automation (CI/CD, scripts)

**Folder layout** is up to your process (e.g. by product, date, or advisory ID). The portal simply mirrors the share’s directory tree from the share root.

### 4.3 Naming and path rules

The backend validates paths with an allowlist-style pattern (alphanumeric, hyphen, underscore, dot, space, forward slash, and URL-encoded segments). Extremely long paths may be rejected (internal limit **2048** characters).

**Recommendations:**

- Use **clear, stable folder names** so support and customers can navigate predictably.
- Prefer **ASCII** file and folder names when possible; if you use spaces or special characters, the webapp will URL-encode them in links.
- Ensure **file extensions** are correct so preview and MIME types behave as expected.

### 4.4 When changes appear in the portal

The app reads **live** from Azure Files. New or updated files appear after **refresh** or when navigating into a folder again; there is no separate “publish” step inside the portal.

---

## 5. Guide for developers

### 5.1 Repository layout

| Path | Purpose |
|------|---------|
| `backend/` | Ballerina package `security_advisories_fileshare` — HTTP API + Azure Files module |
| `webapp/` | React (CRA + `react-app-rewired`) SPA with Asgardeo and Redux |

### 5.2 Prerequisites

- **Ballerina** `2201.12.9` (see `backend/Ballerina.toml`)
- **Node.js** (compatible with `react-scripts` 5 / TypeScript 4.9)

### 5.3 Backend configuration

Configurable values are read from Ballerina config (e.g. `Config.toml` or `Config.toml.local`). Structure:

```toml
[security_advisories_fileshare.file_storage]
fileShareName = "<your-file-share-name>"

[security_advisories_fileshare.file_storage.fileStorageConfig]
accountName = "<storage-account-name>"
accessKeyOrSAS = "<secret>"
authorizationMethod = "accessKey"  # or SAS as applicable
```

Use `Config.toml.local` for local secrets and **exclude** it from git. Never commit real keys.

Run the service (from `backend/`):

```bash
bal run
```

Default listener: **9090**.

**Startup**: The service calls the file share during initialization; if the share is unreachable, the process **fails fast** (check credentials, network, and share name).

### 5.4 Backend HTTP API

Base URL is wherever you host the listener (e.g. `http://localhost:9090` in development).

| Method | Path | Query | Description |
|--------|------|--------|-------------|
| `GET` | `/health` | — | Liveness: file share reachable |
| `GET` | `/directory-content` | `path` optional | List files and folders under `path` (empty = share root). Returns JSON array of items (`name`, `isFolder`, optional `size`, `contentType`). |
| `GET` | `/file` | `path` required | Download file bytes; response includes `Content-Type` and `Content-Disposition: inline` |

Invalid `path` values yield **400**; listing/download failures yield **500** with a generic message in the body.

### 5.5 CORS

`backend/service.bal` configures CORS for local development (`http://localhost:3000`, `http://127.0.0.1:3000`). For other origins, update `allowOrigins` (and redeploy).

### 5.6 Webapp configuration (`config.js`)

The app loads `public/config.js` before the bundle. Define `window.config` with at least:

| Key | Purpose |
|-----|---------|
| `APP_NAME` | Shown in UI / title context |
| `ASGARDEO_BASE_URL` | Asgardeo server URL |
| `ASGARDEO_CLIENT_ID` | OIDC client ID |
| `AUTH_SIGN_IN_REDIRECT_URL` | Post-login redirect URI registered in Asgardeo |
| `AUTH_SIGN_OUT_REDIRECT_URL` | Post-logout redirect |
| `BACKEND_BASE_URL` | API origin (no trailing slash required on paths; client builds `/health`, `/directory-content`, `/file`) |

Example skeleton (replace values):

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

Run the webapp (from `webapp/`):

```bash
npm install
npm start
```

Default dev server: **3000** (matches backend CORS).

### 5.7 Authentication note for production

The SPA enforces login via **Asgardeo**. The backend endpoints shown above do not, in the current code, validate the JWT; they rely on **network placement** (private API, API gateway, mTLS, etc.) in production. Plan accordingly so the file API is not anonymously reachable from the public internet unless you intend that.

### 5.8 Troubleshooting

| Symptom | Things to check |
|--------|------------------|
| Backend won’t start | Share name, account name, key/SAS, firewall rules on the storage account |
| Empty folders in UI | Actual objects in the share under that path; Azure permission of the credential |
| 400 on API | Path characters outside allowed pattern |
| CORS errors in browser | `allowOrigins` includes your webapp origin; HTTPS vs HTTP |
| Preview fails | File missing or renamed on share; large PDF/browser PDF support; ID token still valid |
| Auth loops | Asgardeo redirect URIs match `config.js` exactly (including path and port) |

---

## 6. Summary

- **Publishers** maintain content in **Azure Files**; the portal reflects it in near real time.  
- **Users** sign in with **Asgardeo**, browse the tree, preview PDFs/images, and download anything else.  
- **Developers** configure **Ballerina** + Azure on the server and **`config.js`** + Asgardeo on the client, and align **CORS** and network security with the deployment environment.

For code-level behavior, see `backend/service.bal`, `backend/modules/file_storage/`, and `webapp/src/view/FileExplorer/FileExplorerPage.tsx`.
