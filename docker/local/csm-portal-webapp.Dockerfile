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
# Local-dev image only -- Choreo builds and serves this webapp from source in
# every other environment. Produces a static build served by nginx, with
# /config.js rendered from environment variables at container start (the app
# reads window.config at runtime, never at build time -- see
# src/config/apiConfig.ts and public/config.js.example).
#
# Build context is apps/csm-portal/webapp itself. The two files shared with
# the customer-portal webapp image (the nginx server block and the config.js
# entrypoint hook) come from a second, named build context, `csm-compose`,
# that docker-compose.yml points at scripts/csm-compose/ via
# build.additional_contexts. To build by hand:
#
#   docker build -f docker/local/csm-portal-webapp.Dockerfile \
#     --build-context csm-compose=scripts/csm-compose apps/csm-portal/webapp

FROM node:20-alpine AS builder

WORKDIR /app

RUN corepack enable && corepack prepare pnpm@9 --activate

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY . .
RUN pnpm build

FROM nginx:1.27-alpine

RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid 10110 \
    "csmdev" \
  && mkdir -p /var/cache/nginx /var/run \
  && chown -R csmdev:csmdev /var/cache/nginx /var/run /usr/share/nginx/html \
  # /run is a tmpfs remounted fresh (root-owned) at container start, so a
  # build-time chown of it doesn't stick -- park the pid file somewhere in
  # the writable image layer instead.
  && sed -i 's#pid\s*/run/nginx.pid;#pid /tmp/nginx.pid;#' /etc/nginx/nginx.conf

COPY --from=builder /app/dist /usr/share/nginx/html
COPY --from=csm-compose nginx-spa.conf /etc/nginx/conf.d/default.conf
COPY --from=csm-compose csm-portal-config-entrypoint.sh /docker-entrypoint.d/40-render-config.sh

RUN chmod +x /docker-entrypoint.d/40-render-config.sh \
  && chown -R csmdev:csmdev /usr/share/nginx/html /etc/nginx/conf.d

USER 10110

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8080/config.js >/dev/null || exit 1
