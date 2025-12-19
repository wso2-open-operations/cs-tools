/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly CUSTOMER_PORTAL_ASGARDEO_BASE_URL: string;
  readonly CUSTOMER_PORTAL_ASGARDEO_CLIENT_ID: string;
  readonly CUSTOMER_PORTAL_AUTH_SIGN_IN_REDIRECT_URL: string;
  readonly CUSTOMER_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL: string;
  readonly CUSTOMER_PORTAL_APP_BACKEND_BASE_URL: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
