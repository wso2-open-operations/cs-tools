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

type recordingIngestRepo struct{ got []domain.Registration }

func (r *recordingIngestRepo) Register(_ context.Context, in domain.Registration,
	_ []domain.OrganizationAttribute) (*domain.IngestResult, error) {
	r.got = append(r.got, in)
	return &domain.IngestResult{Status: "created"}, nil
}

// userId is the owner's WSO2 Cloud UUID. Absent is fine (older feeds); present
// and not a UUID fails that registration, and only that one.
func TestIngestUserIDMustBeAUUID(t *testing.T) {
	repo := &recordingIngestRepo{}
	svc := &ingestService{repo: repo}

	reg := func(userID string) domain.IngestRegistration {
		return domain.IngestRegistration{Registration: domain.Registration{CompanyID: "c-1", UserID: userID}}
	}
	res, err := svc.Register(context.Background(), domain.IngestRegistrationsRequest{
		Registrations: []domain.IngestRegistration{
			reg(""),
			reg(" 019a2b3c-4d5e-7f60-8a9b-0c1d2e3f4a5b "),
			reg("moesif-user-42"),
		},
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if res.Accepted != 2 || res.Failed != 1 {
		t.Fatalf("accepted %d, failed %d; want 2 and 1", res.Accepted, res.Failed)
	}
	if !strings.Contains(res.Results[2].Message, "userId") {
		t.Errorf("the refusal does not name userId: %q", res.Results[2].Message)
	}
	if len(repo.got) != 2 || repo.got[1].UserID != "019a2b3c-4d5e-7f60-8a9b-0c1d2e3f4a5b" {
		t.Errorf("repository saw %+v; want the empty and the trimmed UUID only", repo.got)
	}
}
