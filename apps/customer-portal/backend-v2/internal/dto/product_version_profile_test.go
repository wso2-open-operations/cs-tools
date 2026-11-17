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

package dto

import (
	"encoding/json"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// Several product_version rows share one version label and differ only by
// deployment_profile ("API Control Plane" vs "Traffic Manager" for the same
// 4.6.0). Both places the portal shows a version must carry the profile, or
// those rows look identical: the Add Product version list and a deployed
// product's own version.

func profilePtr(s string) *string { return &s }

func TestMapSearchProductVersions_ExposesDeploymentProfile(t *testing.T) {
	in := entity.SearchProductVersionsResponse{
		ProductVersions: []entity.ProductVersionView{
			{ID: "v-cp", Version: "4.6.0", DeploymentProfile: profilePtr("API Control Plane")},
			{ID: "v-tm", Version: "4.6.0", DeploymentProfile: profilePtr("Traffic Manager")},
			{ID: "v-none", Version: "4.6.0"},
		},
		Total: 3, Limit: 10,
	}

	out := MapSearchProductVersions(in)
	if len(out.Versions) != 3 {
		t.Fatalf("got %d versions, want 3", len(out.Versions))
	}
	for i, want := range []*string{profilePtr("API Control Plane"), profilePtr("Traffic Manager"), nil} {
		got := out.Versions[i].DeploymentProfile
		switch {
		case want == nil && got != nil:
			t.Errorf("version %d: deploymentProfile = %q, want nil", i, *got)
		case want != nil && got == nil:
			t.Errorf("version %d: deploymentProfile dropped, want %q", i, *want)
		case want != nil && *got != *want:
			t.Errorf("version %d: deploymentProfile = %q, want %q", i, *got, *want)
		}
	}
}

func TestMapSearchProductVersions_ProfileWireKey(t *testing.T) {
	out := MapSearchProductVersions(entity.SearchProductVersionsResponse{
		ProductVersions: []entity.ProductVersionView{
			{ID: "v-cp", Version: "4.6.0", DeploymentProfile: profilePtr("API Control Plane")},
			{ID: "v-none", Version: "4.6.0"},
		},
	})
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw struct {
		Versions []map[string]any `json:"versions"`
	}
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got, _ := raw.Versions[0]["deploymentProfile"].(string); got != "API Control Plane" {
		t.Errorf("deploymentProfile on the wire = %v, want %q: %s", raw.Versions[0]["deploymentProfile"], "API Control Plane", encoded)
	}
	if _, present := raw.Versions[1]["deploymentProfile"]; present {
		t.Errorf("deploymentProfile should be omitted when the version has none: %s", encoded)
	}
	if got, _ := raw.Versions[0]["version"].(string); got != "4.6.0" {
		t.Errorf("version must stay the plain label, got %q", got)
	}
}

func TestMapSearchDeployedProducts_ExposesVersionProfile(t *testing.T) {
	in := sampleDeployedProduct()
	in.DeployedProducts[0].Version = &entity.DeployedProductVersionRef{
		ID: "v-tm", Name: "4.6.0", DeploymentProfile: profilePtr("Traffic Manager"),
	}

	out := MapSearchDeployedProducts(in)
	v := out.DeployedProducts[0].Version
	if v == nil || v.DeploymentProfile == nil {
		t.Fatalf("version profile dropped: %+v", v)
	}
	if *v.DeploymentProfile != "Traffic Manager" {
		t.Errorf("profile = %q, want %q", *v.DeploymentProfile, "Traffic Manager")
	}
	// The label stays the plain version: case creation and classification
	// build "<product> <version>" strings from it.
	if v.Label != "4.6.0" {
		t.Errorf("label = %q, want the plain version %q", v.Label, "4.6.0")
	}

	encoded, err := json.Marshal(out.DeployedProducts[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw struct {
		Version map[string]any `json:"version"`
	}
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got, _ := raw.Version["deploymentProfile"].(string); got != "Traffic Manager" {
		t.Errorf("version.deploymentProfile on the wire = %v: %s", raw.Version["deploymentProfile"], encoded)
	}
}

func TestMapSearchDeployedProducts_OmitsAbsentVersionProfile(t *testing.T) {
	in := sampleDeployedProduct()
	in.DeployedProducts[0].Version = &entity.DeployedProductVersionRef{ID: "v-1", Name: "4.6.0"}

	encoded, err := json.Marshal(MapSearchDeployedProducts(in).DeployedProducts[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw struct {
		Version map[string]any `json:"version"`
	}
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, present := raw.Version["deploymentProfile"]; present {
		t.Errorf("deploymentProfile should be omitted when absent: %s", encoded)
	}
}

// TestDeployedProductVersionRefUnmarshal_DecodesProfile guards the
// hand-written UnmarshalJSON: it decodes into a shadow struct, so a field
// added to the type but not to that struct is silently dropped.
func TestDeployedProductVersionRefUnmarshal_DecodesProfile(t *testing.T) {
	var withProfile entity.DeployedProductVersionRef
	if err := json.Unmarshal([]byte(`{"id":"v-1","name":"4.6.0","releasedDate":null,"supportEoLDate":null,"deploymentProfile":"Universal Gateway"}`), &withProfile); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if withProfile.DeploymentProfile == nil || *withProfile.DeploymentProfile != "Universal Gateway" {
		t.Errorf("profile not decoded: %+v", withProfile.DeploymentProfile)
	}

	// An older entity-service sends no such key, and a null is a version
	// with no profile: both must decode as nil.
	for _, body := range []string{`{"id":"v-1","name":"4.6.0"}`, `{"id":"v-1","name":"4.6.0","deploymentProfile":null}`} {
		var v entity.DeployedProductVersionRef
		if err := json.Unmarshal([]byte(body), &v); err != nil {
			t.Fatalf("unmarshal %s: %v", body, err)
		}
		if v.DeploymentProfile != nil {
			t.Errorf("%s: profile = %q, want nil", body, *v.DeploymentProfile)
		}
	}
}
