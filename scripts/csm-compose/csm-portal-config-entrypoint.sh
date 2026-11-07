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
# Runs as an nginx image /docker-entrypoint.d/ hook: renders public config.js
# from environment variables set in docker-compose.yml, since this app reads
# window.config at browser runtime (see public/config.js.example), never at
# build time. All values here are local-dev placeholders, never real secrets.
set -eu

cat > /usr/share/nginx/html/config.js <<EOF
window.config = {
  CSM_PORTAL_AUTH_BASE_URL: "${CSM_PORTAL_AUTH_BASE_URL}",
  CSM_PORTAL_AUTH_CLIENT_ID: "${CSM_PORTAL_AUTH_CLIENT_ID}",
  CSM_PORTAL_AUTH_SIGN_IN_REDIRECT_URL: "${CSM_PORTAL_AUTH_SIGN_IN_REDIRECT_URL}",
  CSM_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL: "${CSM_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL}",
  CSM_PORTAL_BACKEND_BASE_URL: "${CSM_PORTAL_BACKEND_BASE_URL}",
  CSM_PORTAL_STREAM_ENABLED: false,
  CSM_PORTAL_THEME: "acrylicOrange",
  CSM_PORTAL_LOG_LEVEL: "DEBUG",
  CSM_PORTAL_FEATURE_OVERRIDES: {},
  CSM_PORTAL_USE_MOCKS: false,
  CSM_PORTAL_ALLOW_MOCK_TOGGLE: false,
  CSM_PORTAL_MAINTENANCE_BANNER_VISIBLE: false,
  CSM_PORTAL_TOP_BANNER_ENABLED: false,
  CSM_PORTAL_MOBILE_APP_PROMPT_ENABLED: false,
};
EOF

echo "[csm-portal webapp] rendered /usr/share/nginx/html/config.js"
