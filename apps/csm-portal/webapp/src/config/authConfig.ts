// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import type { TopBannerItem } from "@config/topBannersConfig";

// Extend window interface to include our config
declare global {
  interface Window {
    config: {
      CSM_PORTAL_AUTH_BASE_URL: string;
      CSM_PORTAL_AUTH_CLIENT_ID: string;
      /**
       * Optional. Where the IdP sends the browser after sign-in / sign-out.
       * Unset means the SDK uses the page's own origin
       * (`window.location.origin`), so the app works on whatever host and port
       * it is served from; the origin it runs on still has to be registered as
       * a redirect URI on the IdP application. Set one only to override that,
       * e.g. behind a proxy whose public origin differs from the browser's.
       */
      CSM_PORTAL_AUTH_SIGN_IN_REDIRECT_URL?: string;
      CSM_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL?: string;
      CSM_PORTAL_BACKEND_BASE_URL: string;
      /**
       * TEMPORARY / LOCAL DEV ONLY. Real Asgardeo sign-in still happens —
       * this does not touch authentication — it only bypasses the
       * post-sign-in *authorization* checks that currently show "You don't
       * have access to this portal yet" for an account the real staging
       * backend hasn't provisioned a portal role for yet: AuthGuard's
       * GET /users/me role check and portalAccess.ts's per-section
       * capabilities (Operations, Time Cards, Escalate, Download, Write).
       * Grep for `devBypassAccessCheck` to find every call site. Never set
       * this in a committed config.js — see @config/devFlags.ts's export
       * for the full explanation.
       */
      CSM_PORTAL_DEV_BYPASS_ACCESS_CHECK?: boolean;
      /**
       * Base URL for the case-activity SSE stream (csm-portal-backend's
       * dedicated :9092 listener, exposed as its own Choreo REST endpoint).
       * Optional — see apiConfig.ts's STREAM_BASE_URL.
       */
      CSM_PORTAL_STREAM_BASE_URL?: string;
      /**
       * Master on/off switch for the case-activity SSE stream, independent
       * of the URL above. Defaults to false when absent — see
       * apiConfig.ts's STREAM_ENABLED.
       */
      CSM_PORTAL_STREAM_ENABLED?: boolean;
      CSM_PORTAL_THEME: string;
      CSM_PORTAL_LOG_LEVEL: string;
      /**
       * Optional. Product name shown beside the logo in the header and in the
       * browser tab. Defaults to "CSM Portal" when unset — see
       * `appearanceConfig.ts`'s `appName`.
       */
      CSM_PORTAL_APP_NAME?: string;
      /**
       * Optional. Header and tab-icon logo for the light colour scheme — a
       * path under `public/` or an absolute URL. Also used for dark mode when
       * `CSM_PORTAL_LOGO_DARK_MODE_URL` is unset. Defaults to "/logo-dark.svg"
       * — see `appearanceConfig.ts`'s `logoLightModeUrl`.
       */
      CSM_PORTAL_LOGO_LIGHT_MODE_URL?: string;
      /**
       * Optional. Header and tab-icon logo for the dark colour scheme.
       * Defaults to "/logo-white.svg" — see `appearanceConfig.ts`'s
       * `logoDarkModeUrl`.
       */
      CSM_PORTAL_LOGO_DARK_MODE_URL?: string;
      /**
       * Per-page visibility overrides keyed by nav-node id (see
       * `csmNavItems.ts`). Object literal, or the same object as a JSON string
       * for platforms that inject config as strings. Pages left out are
       * enabled. See `featureFlags.ts`.
       */
      CSM_PORTAL_FEATURE_OVERRIDES?:
        | Record<string, "enabled" | "wip" | "hidden">
        | string;
      CSM_PORTAL_MAINTENANCE_BANNER_VISIBLE: boolean;
      CSM_PORTAL_MAINTENANCE_BANNER_SEVERITY?: string;
      CSM_PORTAL_MAINTENANCE_BANNER_TITLE?: string;
      CSM_PORTAL_MAINTENANCE_BANNER_MESSAGE?: string;
      CSM_PORTAL_MAINTENANCE_BANNER_ACTION_LABEL?: string;
      CSM_PORTAL_MAINTENANCE_BANNER_ACTION_URL?: string;
      CSM_PORTAL_CHATBOT_WEBSOCKET_URL?: string;
      /**
       * Destination for the header's "Report an issue" control
       * (ReportIssueButton.tsx). Optional; unset hides the control entirely
       * rather than offering a link with nowhere to go.
       */
      CSM_PORTAL_REPORT_ISSUE_URL?: string;
      CSM_PORTAL_FLOATING_NOVERA_ENABLED?: boolean;
      /** Legacy single banner: gates CSM_PORTAL_TOP_BANNER_HTML when it is a string. */
      CSM_PORTAL_TOP_BANNER_ENABLED?: boolean;
      /**
       * Legacy single banner. A raw HTML string (shown only when
       * CSM_PORTAL_TOP_BANNER_ENABLED is true; has no start or expiry) or a banner
       * object (honours its own startsAt and expiresAt).
       * Prefer CSM_PORTAL_TOP_BANNERS.
       */
      CSM_PORTAL_TOP_BANNER_HTML?: string | TopBannerItem;
      /** Ordered banners rendered above the header. Default: []. */
      CSM_PORTAL_TOP_BANNERS?: TopBannerItem[];
      CSM_PORTAL_ANNOUNCEMENT_BANNER_VISIBLE?: boolean;
      CSM_PORTAL_ANNOUNCEMENT_BANNER_STORAGE_KEY?: string;
      CSM_PORTAL_ANNOUNCEMENT_BANNER_HTML?: string;
      /**
       * Dismissible banner nudging CS engineers on a detected mobile phone
       * (and optionally tablet) browser toward the WSO2 Super App micro-app
       * instead of the full web portal. Unlike the customer portal's
       * equivalent, this never blocks access -- engineers may need emergency
       * mobile access. See `mobileAppConfig.ts`.
       */
      CSM_PORTAL_MOBILE_APP_PROMPT_ENABLED?: boolean;
      CSM_PORTAL_MOBILE_APP_IOS_STORE_URL?: string;
      CSM_PORTAL_MOBILE_APP_ANDROID_STORE_URL?: string;
      CSM_PORTAL_MOBILE_APP_INCLUDE_TABLETS?: boolean;
      /**
       * Project key an announcement's "Send test" dry run creates its one
       * real test case in (mirrors the ServiceNow flow's hardcoded
       * `Project Key = DCPSUB` dry-run scoping). Optional — see
       * announcementDryRunConfig.ts's DRY_RUN_TEST_PROJECT_KEY, which
       * defaults to "DCPSUB" when this is unset.
       */
      CSM_PORTAL_ANNOUNCEMENT_TEST_PROJECT_KEY?: string;
      /**
       * Customer-onboarding status column on a project's Contacts tab. Only
       * `true` (or the string `"true"`) turns it on; off is the default and
       * means the column is not rendered and no request is made. Same flag
       * name as the backend env var gating the route it calls — see
       * `onboardingStatusConfig.ts`.
       */
      CSM_MIGRATION_ONBOARDING_STATUS_ENABLED?: boolean | string;
      /**
       * Largest file, in megabytes, offered for upload (default 10, the
       * entity-service default). A number or numeric
       * string. See `attachmentLimitsConfig.ts`.
       */
      CSM_PORTAL_ENTITY_ATTACHMENT_MAX_MB?: number | string;
      /**
       * Most image megabytes one comment or description may hold in total, and
       * so the largest single image (default 1). A number or numeric
       * string. See `attachmentLimitsConfig.ts`.
       */
      CSM_PORTAL_INLINE_IMAGE_MAX_MB?: number | string;
      /**
       * How many 8 MiB ranges of one attachment download are fetched at the
       * same time (default 4, kept between 1 and 8; 1 fetches one at a time).
       * A number or numeric string. See `attachmentLimitsConfig.ts`.
       */
      CSM_PORTAL_DOWNLOAD_PARALLELISM?: number | string;
      /**
       * Optional. Save a download straight to a file the person picks, where the
       * browser supports it. On unless `false` (boolean or "false"). See
       * `attachmentLimitsConfig.ts`.
       */
      CSM_PORTAL_DOWNLOAD_TO_DISK?: boolean | string;
    };
  }
}

interface AuthConfig {
  baseUrl: string;
  clientId: string;
  /** Undefined lets AsgardeoProvider default to `window.location.origin`. */
  signInRedirectURL?: string;
  signOutRedirectURL?: string;
}

const getAuthConfig = (): AuthConfig => {
  const config = window.config;
  const baseUrl = config?.CSM_PORTAL_AUTH_BASE_URL;
  const clientId = config?.CSM_PORTAL_AUTH_CLIENT_ID;
  // Blank counts as unset, so a placeholder empty string in config.js does not
  // reach the SDK as an (invalid) redirect URL.
  const signInRedirectURL =
    config?.CSM_PORTAL_AUTH_SIGN_IN_REDIRECT_URL?.trim() || undefined;
  const signOutRedirectURL =
    config?.CSM_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL?.trim() || undefined;

  const missingVars: string[] = [];

  if (!baseUrl) missingVars.push("CSM_PORTAL_AUTH_BASE_URL");
  if (!clientId) missingVars.push("CSM_PORTAL_AUTH_CLIENT_ID");

  if (missingVars.length > 0) {
    throw new Error(
      `Auth Config Error: Missing required configuration: ${missingVars.join(
        ", ",
      )}`,
    );
  }

  return {
    baseUrl,
    clientId,
    signInRedirectURL,
    signOutRedirectURL,
  };
};

export const authConfig = getAuthConfig();

// devBypassAccessCheck (the value for the CSM_PORTAL_DEV_* key declared
// above) lives in devFlags.ts, not here — see that file's own doc comment
// for why.
