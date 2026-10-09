// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// bufferLogger returns a text logger that writes to the returned buffer, for asserting on log output.
func bufferLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, nil)), &buf
}

// setCSMEnv sets every CSM variable except the Default service id, which each test sets itself.
func setCSMEnv(t *testing.T) {
	t.Helper()
	for _, name := range csmEnvVars {
		if name != defaultServiceIDEnv {
			t.Setenv(name, "x")
		}
	}
	t.Setenv(defaultServiceIDEnv, "")
}

func TestCSMClientFromEnv_DefaultServiceEnablesCSM(t *testing.T) {
	setCSMEnv(t)
	t.Setenv(defaultServiceIDEnv, "svc-default")
	logger, _ := bufferLogger()

	if csmClientFromEnv(logger, time.Second) == nil {
		t.Errorf("CSM disabled although every CSM variable, %s included, is set", defaultServiceIDEnv)
	}
}

func TestCSMClientFromEnv_NoDefaultServiceDisablesCSM(t *testing.T) {
	setCSMEnv(t)
	logger, buf := bufferLogger()

	if csmClientFromEnv(logger, time.Second) != nil {
		t.Fatal("CSM enabled without a Default service")
	}
	if !strings.Contains(buf.String(), "missing=["+defaultServiceIDEnv+"]") {
		t.Errorf("want %s reported missing, got: %s", defaultServiceIDEnv, buf.String())
	}
}

func TestWarnRemovedRoutingEnv(t *testing.T) {
	t.Setenv("CSM_DEFAULT_ASSIGNMENT_GROUP_ID", "")
	t.Setenv("CSM_ASSIGNMENT_GROUP_ROUTES", "")
	logger, buf := bufferLogger()
	warnRemovedRoutingEnv(logger)
	if buf.Len() != 0 {
		t.Errorf("warned with nothing set: %s", buf.String())
	}

	t.Setenv("CSM_ASSIGNMENT_GROUP_ROUTES", `{"account:1":"g"}`)
	warnRemovedRoutingEnv(logger)
	if !strings.Contains(buf.String(), "no longer used") || !strings.Contains(buf.String(), "CSM_ASSIGNMENT_GROUP_ROUTES") {
		t.Errorf("want one no-longer-used warning naming the variable, got: %s", buf.String())
	}
}
