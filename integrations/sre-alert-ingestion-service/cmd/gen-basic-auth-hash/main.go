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

// Command gen-basic-auth-hash reads a single password line from stdin and
// prints its bcrypt hash to stdout, so operators can build
// SRE_ALERT_AUTH_USERS entries ("username:bcryptHash") without any other
// tooling.
//
// Always hashes at bcrypt.DefaultCost — not configurable, deliberately.
// internal/middleware.BasicAuth's username-enumeration defense (comparing
// against a fixed dummy hash on every unknown username) only removes the
// timing signal if every real credential's bcrypt cost matches that dummy's;
// a hash generated at a different cost would take a different amount of
// wall-clock time to compare, reopening the same side channel. See
// ParseBasicAuthUsers, which rejects any hash whose cost isn't DefaultCost
// for the same reason.
//
// Usage:
//
//	echo -n 'the-password' | go run ./cmd/gen-basic-auth-hash
package main

import (
	"bufio"
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(os.Stderr, "gen-basic-auth-hash: failed to read password from stdin:", err)
		os.Exit(1)
	}
	password := trimNewline(line)
	if password == "" {
		fmt.Fprintln(os.Stderr, "gen-basic-auth-hash: empty password")
		os.Exit(1)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen-basic-auth-hash: failed to hash password:", err)
		os.Exit(1)
	}

	fmt.Println(string(hash))
}

// trimNewline strips a single trailing "\n" and, if present, a preceding
// "\r" — bufio.Reader.ReadString('\n') includes the delimiter itself, and a
// CRLF-terminated input would otherwise leave a stray "\r" in the password.
func trimNewline(s string) string {
	if n := len(s); n > 0 && s[n-1] == '\n' {
		s = s[:n-1]
	}
	if n := len(s); n > 0 && s[n-1] == '\r' {
		s = s[:n-1]
	}
	return s
}
