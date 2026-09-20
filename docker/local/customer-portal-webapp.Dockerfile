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
# reads window.config at runtime -- see public/config.js.example).
#
# Build context is the repo root (not this directory), so this Dockerfile
# can also pull in scripts/csm-compose/ -- every app source path below is
# therefore prefixed with apps/customer-portal/webapp/.

FROM node:20-alpine AS builder

WORKDIR /app

RUN corepack enable && corepack prepare pnpm@9 --activate

COPY apps/customer-portal/webapp/package.json apps/customer-portal/webapp/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY apps/customer-portal/webapp/ .
# This repo's pnpm-workspace.yaml here has no `packages:` key (it exists only
# for its allowBuilds/minimumReleaseAgeExclude settings), which the pnpm
# version pnpm-lock.yaml pins (9.15.x) refuses to run a build under
# ("packages field missing or empty") -- a real upstream pnpm/config
# incompatibility, not something this Dockerfile should paper over in the
# actual repo file. It is only ever needed for pnpm's own workspace
# resolution, which this single-package image build doesn't use, so it is
# safe to drop inside the image.
RUN rm -f pnpm-workspace.yaml
RUN pnpm build

FROM nginx:1.27-alpine

RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid 10111 \
    "csmdev" \
  && mkdir -p /var/cache/nginx /var/run \
  && chown -R csmdev:csmdev /var/cache/nginx /var/run /usr/share/nginx/html \
  # /run is a tmpfs remounted fresh (root-owned) at container start, so a
  # build-time chown of it doesn't stick -- park the pid file somewhere in
  # the writable image layer instead.
  && sed -i 's#pid\s*/run/nginx.pid;#pid /tmp/nginx.pid;#' /etc/nginx/nginx.conf

COPY --from=builder /app/dist /usr/share/nginx/html
COPY scripts/csm-compose/nginx-spa.conf /etc/nginx/conf.d/default.conf
COPY scripts/csm-compose/customer-portal-config-entrypoint.sh /docker-entrypoint.d/40-render-config.sh

RUN chmod +x /docker-entrypoint.d/40-render-config.sh \
  && chown -R csmdev:csmdev /usr/share/nginx/html /etc/nginx/conf.d

USER 10111

EXPOSE 8080
