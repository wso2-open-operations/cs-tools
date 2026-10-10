#!/bin/sh
# Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
#
# Renders public/config.js from environment variables at container start.
# All values here are local-dev placeholders, never real secrets.
#
# Every environment value is escaped before it is placed inside a JavaScript
# string literal, and the static part of the file is a quoted heredoc, so a
# value containing a double quote, a backslash or a newline cannot break the
# rendered file (which would leave the SPA with no window.config at all).
set -eu

# Fail fast with a named variable rather than serving a page that boots
# without the setting.
: "${CUSTOMER_PORTAL_AUTH_BASE_URL:?must be set}"
: "${CUSTOMER_PORTAL_AUTH_CLIENT_ID:?must be set}"
: "${CUSTOMER_PORTAL_AUTH_SIGN_IN_REDIRECT_URL:?must be set}"
: "${CUSTOMER_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL:?must be set}"
: "${CUSTOMER_PORTAL_BACKEND_BASE_URL:?must be set}"

# json_escape prints $1 escaped for use inside a double-quoted JS/JSON string:
# backslash and double quote are backslash-escaped, newlines become \n.
json_escape() {
  printf '%s' "$1" \
    | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' \
    | sed -e ':a' -e 'N' -e '$!ba' -e 's/\n/\\n/g'
}

{
  printf 'window.config = {\n'
  printf '  CUSTOMER_PORTAL_AUTH_BASE_URL: "%s",\n' "$(json_escape "$CUSTOMER_PORTAL_AUTH_BASE_URL")"
  printf '  CUSTOMER_PORTAL_AUTH_CLIENT_ID: "%s",\n' "$(json_escape "$CUSTOMER_PORTAL_AUTH_CLIENT_ID")"
  printf '  CUSTOMER_PORTAL_AUTH_SIGN_IN_REDIRECT_URL: "%s",\n' "$(json_escape "$CUSTOMER_PORTAL_AUTH_SIGN_IN_REDIRECT_URL")"
  printf '  CUSTOMER_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL: "%s",\n' "$(json_escape "$CUSTOMER_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL")"
  printf '  CUSTOMER_PORTAL_BACKEND_BASE_URL: "%s",\n' "$(json_escape "$CUSTOMER_PORTAL_BACKEND_BASE_URL")"
  cat <<'EOF'
  CUSTOMER_PORTAL_THEME: "acrylicOrange",
  CUSTOMER_PORTAL_LOG_LEVEL: "DEBUG",
  CUSTOMER_PORTAL_MAINTENANCE_BANNER_VISIBLE: false,
  CUSTOMER_PORTAL_FLOATING_NOVERA_ENABLED: false,
  CUSTOMER_PORTAL_NOVERA_TOKEN_REQUEST_ENABLED: false,
  CUSTOMER_PORTAL_NOVERA_FEEDBACK_ENABLED: false,
  CUSTOMER_PORTAL_TOP_BANNERS: [],
  CUSTOMER_PORTAL_ANNOUNCEMENT_BANNER_VISIBLE: false,
  CUSTOMER_PORTAL_MOBILE_APP_PROMPT_ENABLED: false,
};
EOF
} > "${CONFIG_JS_PATH:-/usr/share/nginx/html/config.js}"

echo "[customer-portal webapp] rendered ${CONFIG_JS_PATH:-/usr/share/nginx/html/config.js}"
