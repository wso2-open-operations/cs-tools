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
# This Dockerfile creates an image that contains the sftp-authn-service executable.

FROM golang:1.24-alpine AS builder

# Set the working directory to /app
WORKDIR /app

# Copy the Go source file into the container
COPY . .

# Run unit tests
RUN go test ./...

# Build the Go program inside the container
RUN go build -o sftpgo-authn-service ./cmd/server/main.go

# Use a small Alpine Linux image as the base image for the final container
FROM alpine:latest

# Set the working directory to /app
WORKDIR /app

# Copy the executable from the builder container
COPY --from=builder /app/sftpgo-authn-service .

# Create a user with a known UID/GID within range 10000-20000.
# This is required by Choreo to run the container as a non-root user.
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid 10014 \
    "choreo"
    
# Use the above created unprivileged user
USER 10014

# Set the entrypoint to the executable
ENTRYPOINT ["./sftpgo-authn-service"]
