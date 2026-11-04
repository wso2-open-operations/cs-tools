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

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type stubProductVersionRepo struct {
	got domain.SearchProductVersionsRequest
}

func (r *stubProductVersionRepo) SearchProductVersions(_ context.Context, req domain.SearchProductVersionsRequest) ([]domain.ProductVersion, int, error) {
	r.got = req
	return nil, 0, nil
}

func (r *stubProductVersionRepo) GetProductVersions(context.Context, []string) ([]domain.ProductVersion, error) {
	return nil, nil
}

func TestProductVersionService_SearchProductVersions_DeploymentProfile(t *testing.T) {
	custom, none := "Split", ""
	for name, tc := range map[string]struct {
		in   *string
		want string
	}{
		"omitted applies no profile filter": {nil, ""},
		"explicit value is kept":            {&custom, "Split"},
		"empty applies no profile filter":   {&none, ""},
	} {
		t.Run(name, func(t *testing.T) {
			repo := &stubProductVersionRepo{}
			svc := NewProductVersionService(repo)
			if _, err := svc.SearchProductVersions(context.Background(), domain.SearchProductVersionsRequest{DeploymentProfile: tc.in}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := ""
			if repo.got.DeploymentProfile != nil {
				got = *repo.got.DeploymentProfile
			}
			if got != tc.want {
				t.Errorf("repository deploymentProfile = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestProductVersionService_SearchProductVersions_RejectsOversizedDeploymentProfile(t *testing.T) {
	long := strings.Repeat("x", maxDeploymentProfileLen+1)
	svc := NewProductVersionService(&stubProductVersionRepo{})
	if _, err := svc.SearchProductVersions(context.Background(), domain.SearchProductVersionsRequest{DeploymentProfile: &long}); err == nil {
		t.Error("expected a validation error for an oversized deploymentProfile")
	}
}
