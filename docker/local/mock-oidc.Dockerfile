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
# LOCAL DEV ONLY -- builds the mock OIDC provider (scripts/csm-compose/
# mock-oidc) used by the docker-compose stack in apps/csm-portal/README.md
# so contributors can run the CSM platform without access to a real
# identity provider. The binary refuses to start unless
# MOCK_OIDC_LOCAL_DEV=1 is set; docker-compose.yml sets it.

# Pinned to the patch level the go.mod directive asks for, with toolchain
# downloads disabled, so the build is hermetic.
FROM golang:1.26.6-alpine AS builder

ENV GOTOOLCHAIN=local GOFLAGS=-mod=readonly CGO_ENABLED=0

WORKDIR /app

COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/mock-oidc .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /out/mock-oidc ./mock-oidc

RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid 10109 \
    "csmdev"

USER 10109

ENTRYPOINT ["./mock-oidc"]
