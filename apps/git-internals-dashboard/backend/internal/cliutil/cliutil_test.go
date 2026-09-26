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

package cliutil

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadDotEnvSetsUnsetVariables covers comment/blank-line skipping,
// quote stripping, and the no-overwrite-when-already-set rule, all in one
// .env file.
func TestLoadDotEnvSetsUnsetVariables(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	contents := "" +
		"# a comment line\n" +
		"\n" +
		"FOO=bar\n" +
		"QUOTED=\"double quoted\"\n" +
		"SINGLE='single quoted'\n" +
		"ALREADY_SET=from-file\n" +
		"NOT_KEY_VALUE\n"
	if err := os.WriteFile(envPath, []byte(contents), 0o600); err != nil {
		t.Fatalf("write .env fixture: %v", err)
	}

	t.Setenv("FOO", "")
	t.Setenv("QUOTED", "")
	t.Setenv("SINGLE", "")
	t.Setenv("ALREADY_SET", "from-env")
	t.Setenv("NOT_KEY_VALUE", "")

	LoadDotEnv(envPath)

	cases := map[string]string{
		"FOO":           "bar",
		"QUOTED":        "double quoted",
		"SINGLE":        "single quoted",
		"ALREADY_SET":   "from-env", // pre-set value wins, file value is not applied
		"NOT_KEY_VALUE": "",         // no "=" on the line, so it's skipped rather than set
	}
	for k, want := range cases {
		if got := os.Getenv(k); got != want {
			t.Errorf("%s: expected %q, got %q", k, want, got)
		}
	}
}

// TestLoadDotEnvMissingFileIsANoOp verifies a nonexistent path is silently
// ignored rather than treated as an error.
func TestLoadDotEnvMissingFileIsANoOp(t *testing.T) {
	t.Setenv("CLIUTIL_TEST_MISSING_FILE_VAR", "")
	LoadDotEnv(filepath.Join(t.TempDir(), "does-not-exist.env"))
	if got := os.Getenv("CLIUTIL_TEST_MISSING_FILE_VAR"); got != "" {
		t.Errorf("expected env untouched for a missing .env file, got %q", got)
	}
}
