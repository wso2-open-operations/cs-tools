// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// Package cliutil holds small startup helpers shared by this backend's
// command-line entry points (cmd/server, cmd/seed, cmd/backfill-meta): a
// best-effort .env loader and a required-environment-variable check.
package cliutil

import (
	"bufio"
	"errors"
	"log/slog"
	"os"
	"strings"
)

// LoadDotEnv reads a .env file at path and sets any environment variable
// from it that isn't already set. Silently ignored if the file does not
// exist; a warning is logged via slog for any other read error, since a
// dotenv file that exists but can't be read (e.g. a permissions problem)
// is worth surfacing rather than treating the same as "no .env file".
func LoadDotEnv(path string) {
	f, err := os.Open(path) // #nosec G304 -- path is a caller-supplied config file path, not request input
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("LoadDotEnv: failed to open .env file", "path", path, "err", err)
		}
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("LoadDotEnv: error reading .env file", "path", path, "err", err)
	}
}

// MustEnv returns the value of the given required environment variable,
// logging an error via slog and exiting the process with status 1 if it is
// unset or empty.
func MustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable is not set", "key", key)
		os.Exit(1)
	}
	return v
}
