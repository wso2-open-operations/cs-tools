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

package main

// The CSM Cloud Status API is a second Choreo endpoint whose contract,
// cloud-status.openapi.yaml, copies the cloud status reads from openapi.yaml.
// Choreo publishes only the operations an endpoint's contract lists, and the
// wildcard route when the endpoint names none, so these tests keep the copy
// read-only and in step with openapi.yaml, and the endpoint pointed at it.

import (
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

const (
	mainSpec        = "../../openapi.yaml"
	cloudStatusSpec = "../../cloud-status.openapi.yaml"
)

// readYAML decodes file into out; the decoder also rejects a key given twice.
func readYAML(t *testing.T, file string, out any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Clean(file))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(raw, out); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
}

// openAPISpec is the part of an OpenAPI document these tests compare.
type openAPISpec struct {
	Security   []map[string][]string     `yaml:"security"`
	Paths      map[string]map[string]any `yaml:"paths"`
	Components map[string]map[string]any `yaml:"components"`
}

var (
	httpMethods   = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}
	pathItemOther = []string{"summary", "description", "parameters", "servers"}
)

// TestCloudStatusSpecIsReadOnly fails on any operation in cloud-status.openapi.yaml
// other than a GET, and on a path item key that could hide one.
func TestCloudStatusSpecIsReadOnly(t *testing.T) {
	var s openAPISpec
	readYAML(t, cloudStatusSpec, &s)
	if len(s.Paths) == 0 {
		t.Fatal("cloud-status.openapi.yaml lists no paths")
	}
	for path, item := range s.Paths {
		for key := range item {
			switch {
			case key == "get":
			case slices.Contains(httpMethods, key):
				t.Errorf("%s %s: the cloud status API lists reads only", strings.ToUpper(key), path)
			case !slices.Contains(pathItemOther, key):
				// A $ref here, say, could bring in operations this test never sees.
				t.Errorf("path %s: unexpected key %q", path, key)
			}
		}
	}
}

// isCloudStatusPath reports whether path belongs in the cloud status API:
// the health check and the cloud status reads, nothing else.
func isCloudStatusPath(path string) bool {
	return path == "/health" || strings.HasPrefix(path, "/cloud-status/")
}

// TestCloudStatusSpecCopiesTheMainSpec fails unless the copy holds exactly
// openapi.yaml's /health and /cloud-status/* paths, and its top-level security
// requirement, each of its paths and its components equal openapi.yaml's.
func TestCloudStatusSpecCopiesTheMainSpec(t *testing.T) {
	var main, cloudStatus openAPISpec
	readYAML(t, mainSpec, &main)
	readYAML(t, cloudStatusSpec, &cloudStatus)
	// The top-level requirement is what makes every operation need a token;
	// an operation opts out only with its own (copied) security, as /health does.
	if len(cloudStatus.Security) == 0 || !reflect.DeepEqual(cloudStatus.Security, main.Security) {
		t.Errorf("cloud-status.openapi.yaml security %v, want openapi.yaml's %v (and not empty)",
			cloudStatus.Security, main.Security)
	}
	var want []string
	for path := range main.Paths {
		if isCloudStatusPath(path) {
			want = append(want, path)
		}
	}
	if got := slices.Sorted(maps.Keys(cloudStatus.Paths)); !slices.Equal(got, slices.Sorted(slices.Values(want))) {
		t.Errorf("cloud-status.openapi.yaml paths %v, want openapi.yaml's /health and /cloud-status/* paths %v",
			got, slices.Sorted(slices.Values(want)))
	}
	for path, item := range cloudStatus.Paths {
		if !reflect.DeepEqual(item, main.Paths[path]) {
			t.Errorf("path %s differs from openapi.yaml's; copy it from there", path)
		}
	}
	for kind, entries := range cloudStatus.Components {
		for name, entry := range entries {
			if !reflect.DeepEqual(entry, main.Components[kind][name]) {
				t.Errorf("components.%s.%s differs from openapi.yaml's; copy it from there", kind, name)
			}
		}
	}
}

// collectRefs appends every $ref value found anywhere in v. The decoder turns
// a mapping with any non-string key (an unquoted 200:, say) into map[any]any.
func collectRefs(v any, out *[]string) {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if s, ok := val.(string); ok && k == "$ref" {
				*out = append(*out, s)
			}
			collectRefs(val, out)
		}
	case map[any]any:
		for k, val := range x {
			if s, ok := val.(string); ok && k == "$ref" {
				*out = append(*out, s)
			}
			collectRefs(val, out)
		}
	case []any:
		for _, val := range x {
			collectRefs(val, out)
		}
	}
}

// TestCloudStatusSpecRefsResolve fails on a $ref in cloud-status.openapi.yaml
// that does not resolve inside it, since Choreo publishes that file on its own.
func TestCloudStatusSpecRefsResolve(t *testing.T) {
	var doc map[string]any
	readYAML(t, cloudStatusSpec, &doc)
	var refs []string
	collectRefs(doc, &refs)
	if len(refs) == 0 {
		t.Fatal("cloud-status.openapi.yaml: no $ref found")
	}
	for _, ref := range refs {
		var node any = doc
		for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
			m, _ := node.(map[string]any)
			node = m[part]
		}
		// The walk follows string-keyed maps only, so a $ref into anything else
		// fails here rather than passing unchecked.
		if !strings.HasPrefix(ref, "#/") || node == nil {
			t.Errorf("%s: does not resolve within cloud-status.openapi.yaml", ref)
		}
	}
}

// TestChoreoEndpointsNameTheirContracts fails unless component.yaml declares
// exactly the two endpoints, each on the service's port with its own
// contract; an endpoint without one would publish the wildcard route.
func TestChoreoEndpointsNameTheirContracts(t *testing.T) {
	var descriptor struct {
		Endpoints []struct {
			Name    string `yaml:"name"`
			Service struct {
				Port int `yaml:"port"`
			} `yaml:"service"`
			SchemaFilePath string `yaml:"schemaFilePath"`
		} `yaml:"endpoints"`
	}
	readYAML(t, "../../.choreo/component.yaml", &descriptor)
	type endpoint struct{ port, schema string }
	got := map[string]endpoint{}
	for _, e := range descriptor.Endpoints {
		got[e.Name] = endpoint{strconv.Itoa(e.Service.Port), e.SchemaFilePath}
	}
	// Both endpoints are the one listener on PORT's default (main.go).
	want := map[string]endpoint{
		"csm-integration-service-api": {"8080", "openapi.yaml"},
		"csm-cloud-status-api":        {"8080", "cloud-status.openapi.yaml"},
	}
	if len(descriptor.Endpoints) != len(want) || !maps.Equal(got, want) {
		t.Errorf("component.yaml endpoints %+v, want %v", descriptor.Endpoints, want)
	}
}
