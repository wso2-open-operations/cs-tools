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
# Runs the Playwright E2E suite in a container.
#
# Lives at the app root, so the build context is this directory and every COPY
# below is prefixed `webapp/`. COPY resolves against the context, never against
# where this file sits — so the prefix is what changes if the file moves, not the
# other way round.
#
# The image mirrors the repo layout (/app/webapp), leaving room for sibling units
# if backend/ or microapp/ ever gain tests. Today neither has any — backend/ has
# no *_test.bal and microapp/ no test script — so webapp's Playwright suite is
# the whole of this app's end-to-end coverage. That is a finding, not an
# omission.
#
# Build. No -f needed: a file named Dockerfile at the context root is the
# default.
#   cd apps/customer-portal
#   docker build -t customer-portal-e2e .
#
# The image tag tracks @playwright/test in webapp/package.json — 1.62.0.
# Playwright refuses to drive browsers built for a different release, so the two
# are bumped together; a mismatch fails at launch rather than subtly.
#
# The suite targets a deployed environment rather than serving the app here:
# webapp/.env.e2e (tracked, copied in) already sets
# E2E_BASE_URL=https://support-stg.wso2.com and E2E_NO_WEBSERVER=1, so no
# environment variables are required to run. Serving it locally instead would
# need webapp/public/config.js, which is git-ignored and deliberately not copied.
#
# Run. The captured session is mounted, never baked in: it holds real tokens, and
# .dockerignore keeps it out of the build context for that reason.
#   docker run --rm \
#     -v "$PWD/webapp/tests/e2e/storageState:/app/webapp/tests/e2e/storageState" \
#     -v "$PWD/webapp/playwright-report:/app/webapp/playwright-report" \
#     -v "$PWD/webapp/test-results:/app/webapp/test-results" \
#     customer-portal-e2e
#
# Mounted read-write, unlike an ordinary fixture: the auth setup project signs in
# and WRITES session.json back. Add :ro only when running with credentials unset,
# so the suite replays an existing session instead of minting one.
#
# To sign in from the container, pass credentials at run time — never build them
# in:
#   docker run --rm --env-file webapp/.env.e2e.local … customer-portal-e2e
#
# Anything after the image name replaces the default command, so a subset runs
# as:
#   docker run … customer-portal-e2e pnpm exec playwright test specs/chat/
#
# THE REPORT. Playwright writes an HTML report to playwright-report/. Mounting
# that directory out (above) is the simplest way to read it:
#   npx playwright show-report webapp/playwright-report
#
# It can also be emailed on every run. Set MAIL_TO plus SMTP credentials and the
# entrypoint sends the report — passed or failed — as a gzipped tarball:
#   docker run --rm \
#     -e MAIL_TO=paraparan@wso2.com \
#     -e SMTP_URL=smtps://smtp.example.com:465 \
#     -e SMTP_USER=… -e SMTP_PASSWORD=… \
#     … customer-portal-e2e
#
# With MAIL_TO unset it is skipped silently, so the image works unchanged
# without mail configuration. Credentials are passed at run time and never built
# in — prefer --env-file over -e so they stay out of shell history and
# `docker inspect`.
#
# One caveat worth knowing: these specs run against a real backend and create
# permanent records (cases, chats, escalations). A container makes it easy to run
# them repeatedly — each run leaves real data behind in whatever environment
# E2E_BASE_URL names.
#

FROM mcr.microsoft.com/playwright:v1.62.0-noble

# The suite runs from /app/webapp; the extra level is what keeps room for
# sibling units.
WORKDIR /app/webapp

# Matches the pnpm used on the host, so the install resolves the same tree the
# lockfile was written against. webapp carries its own lockfile and is built
# independently of the other units — hence pnpm here and npm in microapp.
RUN corepack enable && corepack prepare pnpm@10.32.1 --activate

# Dependencies as their own layer, before the source: they change far less often
# than the specs do, so editing a test does not trigger a reinstall.
COPY webapp/package.json webapp/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

RUN addgroup --gid 10014 updates && \
    adduser  --disabled-password  --no-create-home --uid 10014 --ingroup updates updatesuser

USER 10014
# The whole webapp, not just tests/: the specs resolve imports through the path
# aliases in tsconfig, and playwright.config.ts reads .env.e2e from the webapp
# root.
COPY webapp/ ./

# Never boot a dev server in here. Real env beats .env.e2e in the config's
# precedence chain, so this holds even if that tracked file is changed to target
# localhost for host runs — the image could not serve the app anyway, since
# public/config.js is git-ignored and not copied.
#
# E2E_BASE_URL is deliberately NOT given a default: baking an environment into
# an image is how a container ends up writing to the wrong tenant. It comes from
# .env.e2e or from -e at run time, and the entrypoint rejects an unreachable
# one (localhost) with an actionable message rather than letting every test fail
# on a connection error.
ENV E2E_NO_WEBSERVER=1

# Relative to WORKDIR (/app/webapp), where `COPY webapp/ ./` put the webapp's
# contents — so NOT `webapp/node_modules/...`, which would resolve to
# /app/webapp/webapp and fail at startup.
CMD ["node_modules/.bin/playwright", "test"]
