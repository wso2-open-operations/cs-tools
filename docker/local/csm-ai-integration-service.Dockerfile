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
# Local-dev image for this service. Choreo deploys it from source via its
# own buildpack in every other environment -- this Dockerfile exists only
# so the service can run in the docker-compose stack described in
# apps/csm-portal/README.md.

FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /out/csm-ai-integration-service ./cmd/server

FROM alpine:3.20

# ca-certificates: this service calls the model gateway over HTTPS and needs a
# real trust store to verify it.
RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /out/csm-ai-integration-service ./csm-ai-integration-service

# The LLM call log's directory (LLM_CALL_LOG_DIR, local development only),
# writable by the service's user whether or not a host directory is mounted.
RUN mkdir -p /app/llm-logs && chown 10108 /app/llm-logs

RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid 10108 \
    "csmdev"

USER 10108

ENTRYPOINT ["./csm-ai-integration-service"]
