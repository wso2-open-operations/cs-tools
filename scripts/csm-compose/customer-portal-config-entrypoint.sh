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
set -eu

cat > /usr/share/nginx/html/config.js <<EOF
window.config = {
  CUSTOMER_PORTAL_AUTH_BASE_URL: "${CUSTOMER_PORTAL_AUTH_BASE_URL}",
  CUSTOMER_PORTAL_AUTH_CLIENT_ID: "${CUSTOMER_PORTAL_AUTH_CLIENT_ID}",
  CUSTOMER_PORTAL_AUTH_SIGN_IN_REDIRECT_URL: "${CUSTOMER_PORTAL_AUTH_SIGN_IN_REDIRECT_URL}",
  CUSTOMER_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL: "${CUSTOMER_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL}",
  CUSTOMER_PORTAL_BACKEND_BASE_URL: "${CUSTOMER_PORTAL_BACKEND_BASE_URL}",
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

echo "[customer-portal webapp] rendered /usr/share/nginx/html/config.js"
