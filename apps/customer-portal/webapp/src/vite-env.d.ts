/// <parameter name="reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_ASGARDEO_BASE_URL: string;
  readonly VITE_ASGARDEO_CLIENT_ID: string;
  readonly VITE_AUTH_SIGN_IN_REDIRECT_URL: string;
  readonly VITE_AUTH_SIGN_OUT_REDIRECT_URL: string;
  readonly VITE_REACT_APP_BACKEND_BASE_URL: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
