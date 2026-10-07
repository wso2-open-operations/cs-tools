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
# Local-dev image for scripts/csm-compose/seed-generator -- see that
# package's doc comment and apps/csm-portal/README.md. LOCAL DEVELOPMENT
# ONLY: this generates randomized dummy CSM data into entity-service's
# Postgres database and is never built or deployed anywhere other than this
# docker-compose stack.

# Pinned to the patch level the go.mod directive asks for, with toolchain
# downloads disabled, so the build is hermetic.
FROM golang:1.26.6-alpine AS builder

ENV GOTOOLCHAIN=local GOFLAGS=-mod=readonly CGO_ENABLED=0

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/seed-generator .

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /out/seed-generator ./seed-generator

RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid 10112 \
    "csmdev"

USER 10112

ENTRYPOINT ["./seed-generator"]
