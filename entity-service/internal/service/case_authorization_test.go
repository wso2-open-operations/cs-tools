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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// scopedGetCaseByID simulates the real repository's project-scoped
// GetCaseByID: success for an unrestricted (internal) caller or a caller
// whose scope includes allowedProjectID, NotFoundError otherwise -- the
// same semantics the real SQL enforces. Used to prove that
// caseService.authorizeCaseAccess (and every case-adjacent operation that
// calls it) actually propagates the caller's resolved scope into the gate,
// not just that it compiles.
func scopedGetCaseByID(allowedProjectID string) func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
	return func(_ context.Context, id string, scope repository.SearchScope) (domain.CaseView, error) {
		if scope.Unrestricted {
			return domain.CaseView{ID: id}, nil
		}
		for _, p := range scope.ProjectIDs {
			if p == allowedProjectID {
				return domain.CaseView{ID: id}, nil
			}
		}
		return domain.CaseView{}, &apierror.NotFoundError{Msg: "case not found"}
	}
}

const authTestProject = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

var inScopeAccess = stubAccess{scope: AccessScope{ProjectIDs: []string{authTestProject}}}
var outOfScopeAccess = stubAccess{scope: AccessScope{ProjectIDs: []string{"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"}}}

// TestCaseService_CreateCaseComment_ScopesToCallerProjects is the core
// regression guard: CreateCaseComment applied no per-resource authorization
// at all before this fix -- any authenticated caller could comment on any
// case by id, regardless of project.
func TestCaseService_CreateCaseComment_ScopesToCallerProjects(t *testing.T) {
	req := domain.CreateCaseCommentRequest{CaseID: testCaseID, Type: domain.CommentTypeComment, Content: "hello"}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	t.Run("out of scope is denied, repo write never reached", func(t *testing.T) {
		repo := &stubCaseRepo{getCaseByID: scopedGetCaseByID(authTestProject)}
		svc := NewCaseService(repo, actorUserRepo(t), nil, outOfScopeAccess)
		_, err := svc.CreateCaseComment(ctx, req)
		var nf *apierror.NotFoundError
		if !asNotFoundError(err, &nf) {
			t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("in scope succeeds, reaches repo", func(t *testing.T) {
		called := false
		repo := &stubCaseRepo{
			getCaseByID: scopedGetCaseByID(authTestProject),
			createCaseComment: func(context.Context, domain.CreateCaseCommentRequest) (domain.CaseComment, error) {
				called = true
				return domain.CaseComment{ID: "c1"}, nil
			},
		}
		svc := NewCaseService(repo, actorUserRepo(t), nil, inScopeAccess)
		if _, err := svc.CreateCaseComment(ctx, req); err != nil {
			t.Fatalf("CreateCaseComment: %v", err)
		}
		if !called {
			t.Fatal("expected repo.CreateCaseComment to be called")
		}
	})
}

// TestCaseService_SearchCaseComments_ScopesToCallerProjects mirrors
// CreateCaseComment's guard for the read path.
func TestCaseService_SearchCaseComments_ScopesToCallerProjects(t *testing.T) {
	req := domain.SearchCaseCommentsRequest{CaseID: testCaseID, Pagination: domain.Pagination{Limit: 10}}
	ctx := context.Background()

	t.Run("out of scope is denied, repo read never reached", func(t *testing.T) {
		repo := &stubCaseRepo{getCaseByID: scopedGetCaseByID(authTestProject)}
		svc := NewCaseService(repo, stubUserRepo{}, nil, outOfScopeAccess)
		_, err := svc.SearchCaseComments(ctx, req)
		var nf *apierror.NotFoundError
		if !asNotFoundError(err, &nf) {
			t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("in scope succeeds, reaches repo", func(t *testing.T) {
		called := false
		repo := &stubCaseRepo{
			getCaseByID: scopedGetCaseByID(authTestProject),
			searchCaseComments: func(context.Context, domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error) {
				called = true
				return []domain.CaseComment{}, 0, nil
			},
		}
		svc := NewCaseService(repo, stubUserRepo{}, nil, inScopeAccess)
		if _, err := svc.SearchCaseComments(ctx, req); err != nil {
			t.Fatalf("SearchCaseComments: %v", err)
		}
		if !called {
			t.Fatal("expected repo.SearchCaseComments to be called")
		}
	})
}

// TestCaseService_UpdateCase_ScopesToCallerProjects proves the single gate
// at the top of UpdateCase covers its plain state/severity/workState-less
// field-update branch -- none of UpdateCase's branches had any per-resource
// authorization at all before this fix.
func TestCaseService_UpdateCase_ScopesToCallerProjects(t *testing.T) {
	req := domain.UpdateCaseRequest{ID: testCaseID, Subject: str("new subject")}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	t.Run("out of scope is denied, repo write never reached", func(t *testing.T) {
		repo := &stubCaseRepo{getCaseByID: scopedGetCaseByID(authTestProject)}
		svc := NewCaseService(repo, actorUserRepo(t), nil, outOfScopeAccess)
		_, err := svc.UpdateCase(ctx, req)
		var nf *apierror.NotFoundError
		if !asNotFoundError(err, &nf) {
			t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("in scope succeeds, reaches repo", func(t *testing.T) {
		called := false
		repo := &stubCaseRepo{
			getCaseByID: scopedGetCaseByID(authTestProject),
			updateCaseFields: func(_ context.Context, req domain.UpdateCaseRequest, actorID, actorEmail string) (time.Time, error) {
				called = true
				return time.Now(), nil
			},
		}
		svc := NewCaseService(repo, actorUserRepo(t), nil, inScopeAccess)
		if _, err := svc.UpdateCase(ctx, req); err != nil {
			t.Fatalf("UpdateCase: %v", err)
		}
		if !called {
			t.Fatal("expected repo.UpdateCaseFields to be called")
		}
	})
}

// TestCaseService_CreateCaseAttachment_ScopesToCallerProjects covers the
// attachment-create path, keyed directly by req.ReferenceID (the case id).
func TestCaseService_CreateCaseAttachment_ScopesToCallerProjects(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := validCreateAttachmentRequest()
	req.ReferenceID = testCaseID

	t.Run("out of scope is denied, repo write never reached", func(t *testing.T) {
		repo := &stubCaseRepo{getCaseByID: scopedGetCaseByID(authTestProject)}
		svc := NewCaseService(repo, actorUserRepo(t), nil, outOfScopeAccess)
		_, err := svc.CreateCaseAttachment(ctx, req)
		var nf *apierror.NotFoundError
		if !asNotFoundError(err, &nf) {
			t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("in scope succeeds, reaches repo", func(t *testing.T) {
		called := false
		repo := &stubCaseRepo{
			getCaseByID: scopedGetCaseByID(authTestProject),
			createCaseAttachment: func(_ context.Context, req domain.CreateAttachmentRequest) (domain.Attachment, error) {
				called = true
				return domain.Attachment{ID: testAttachmentID, ReferenceID: req.ReferenceID}, nil
			},
		}
		svc := NewCaseService(repo, actorUserRepo(t), nil, inScopeAccess)
		if _, err := svc.CreateCaseAttachment(ctx, req); err != nil {
			t.Fatalf("CreateCaseAttachment: %v", err)
		}
		if !called {
			t.Fatal("expected repo.CreateCaseAttachment to be called")
		}
	})
}

// TestCaseService_SearchCaseAttachments_ScopesToCallerProjects mirrors
// CreateCaseAttachment's guard for the read path.
func TestCaseService_SearchCaseAttachments_ScopesToCallerProjects(t *testing.T) {
	req := domain.SearchAttachmentsRequest{ReferenceID: testCaseID, ReferenceType: domain.ReferenceTypeCase, Pagination: domain.Pagination{Limit: 10}}
	ctx := context.Background()

	t.Run("out of scope is denied, repo read never reached", func(t *testing.T) {
		repo := &stubCaseRepo{getCaseByID: scopedGetCaseByID(authTestProject)}
		svc := NewCaseService(repo, stubUserRepo{}, nil, outOfScopeAccess)
		_, err := svc.SearchCaseAttachments(ctx, req)
		var nf *apierror.NotFoundError
		if !asNotFoundError(err, &nf) {
			t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("in scope succeeds, reaches repo", func(t *testing.T) {
		called := false
		repo := &stubCaseRepo{
			getCaseByID: scopedGetCaseByID(authTestProject),
			searchCaseAttachments: func(context.Context, string, domain.Pagination) ([]domain.Attachment, int, error) {
				called = true
				return []domain.Attachment{}, 0, nil
			},
		}
		svc := NewCaseService(repo, stubUserRepo{}, nil, inScopeAccess)
		if _, err := svc.SearchCaseAttachments(ctx, req); err != nil {
			t.Fatalf("SearchCaseAttachments: %v", err)
		}
		if !called {
			t.Fatal("expected repo.SearchCaseAttachments to be called")
		}
	})
}

// TestCaseService_SearchCaseActivities_ScopesToCallerProjects covers the
// case activity feed, keyed directly by req.CaseID.
func TestCaseService_SearchCaseActivities_ScopesToCallerProjects(t *testing.T) {
	req := domain.SearchCaseActivitiesRequest{CaseID: testCaseID, Pagination: domain.Pagination{Limit: 10}}
	ctx := context.Background()

	t.Run("out of scope is denied, repo read never reached", func(t *testing.T) {
		repo := &stubCaseRepo{getCaseByID: scopedGetCaseByID(authTestProject)}
		svc := NewCaseService(repo, stubUserRepo{}, nil, outOfScopeAccess)
		_, err := svc.SearchCaseActivities(ctx, req)
		var nf *apierror.NotFoundError
		if !asNotFoundError(err, &nf) {
			t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("in scope succeeds, reaches repo", func(t *testing.T) {
		called := false
		repo := &stubCaseRepo{
			getCaseByID: scopedGetCaseByID(authTestProject),
			searchCaseActivities: func(context.Context, domain.SearchCaseActivitiesRequest) ([]domain.CaseActivity, int, error) {
				called = true
				return []domain.CaseActivity{}, 0, nil
			},
		}
		svc := NewCaseService(repo, stubUserRepo{}, nil, inScopeAccess)
		if _, err := svc.SearchCaseActivities(ctx, req); err != nil {
			t.Fatalf("SearchCaseActivities: %v", err)
		}
		if !called {
			t.Fatal("expected repo.SearchCaseActivities to be called")
		}
	})
}

// TestCaseService_AddCaseTag_ScopesToCallerProjects covers AddCaseTag/
// addCaseTagAs, keyed directly by caseID.
func TestCaseService_AddCaseTag_ScopesToCallerProjects(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	t.Run("out of scope is denied, repo write never reached", func(t *testing.T) {
		repo := &stubCaseRepo{getCaseByID: scopedGetCaseByID(authTestProject)}
		svc := NewCaseService(repo, actorUserRepo(t), nil, outOfScopeAccess)
		_, err := svc.AddCaseTag(ctx, testCaseID, "urgent")
		var nf *apierror.NotFoundError
		if !asNotFoundError(err, &nf) {
			t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("in scope succeeds, reaches repo", func(t *testing.T) {
		called := false
		repo := &stubCaseRepo{
			getCaseByID: scopedGetCaseByID(authTestProject),
			addCaseTag: func(_ context.Context, caseID, label, actorEmail string) (domain.Tag, error) {
				called = true
				return domain.Tag{ID: "t1", Label: label}, nil
			},
		}
		svc := NewCaseService(repo, actorUserRepo(t), nil, inScopeAccess)
		if _, err := svc.AddCaseTag(ctx, testCaseID, "urgent"); err != nil {
			t.Fatalf("AddCaseTag: %v", err)
		}
		if !called {
			t.Fatal("expected repo.AddCaseTag to be called")
		}
	})
}

// TestCaseService_RemoveCaseTag_ScopesToCallerProjects covers RemoveCaseTag,
// keyed directly by caseID (plus tagID, unrelated to project scoping).
func TestCaseService_RemoveCaseTag_ScopesToCallerProjects(t *testing.T) {
	const tagID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	t.Run("out of scope is denied, repo write never reached", func(t *testing.T) {
		repo := &stubCaseRepo{getCaseByID: scopedGetCaseByID(authTestProject)}
		svc := NewCaseService(repo, actorUserRepo(t), nil, outOfScopeAccess)
		err := svc.RemoveCaseTag(ctx, testCaseID, tagID)
		var nf *apierror.NotFoundError
		if !asNotFoundError(err, &nf) {
			t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
		}
	})
}

// TestCaseService_AttachmentByIDOperations_ScopeToOwningCaseProjects covers
// the attachment-id-keyed operations (Confirm/Delete/Get/Update), each of
// which resolves the attachment's real owning case first, then applies the
// same case-scope gate. These previously had no per-resource authorization
// at all -- see e.g. ConfirmCaseAttachment/SearchCaseAttachments' own doc
// comments ("any authenticated user may perform on any case attachment").
func TestCaseService_AttachmentByIDOperations_ScopeToOwningCaseProjects(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	key := testStorageKey

	t.Run("ConfirmCaseAttachment", func(t *testing.T) {
		t.Run("out of scope is denied, repo mutation never reached", func(t *testing.T) {
			repo := &stubCaseRepo{
				getCaseByID: scopedGetCaseByID(authTestProject),
				getCaseAttachmentByID: func(_ context.Context, id string) (domain.Attachment, error) {
					return domain.Attachment{ID: id, ReferenceID: testCaseID, Status: domain.AttachmentStatusPending, StorageKey: &key, CreatedBy: domain.NewUserReference("user-jane", "jane.doe@example.com", "Jane Doe")}, nil
				},
			}
			svc := NewCaseService(repo, actorUserRepo(t), nil, outOfScopeAccess)
			_, err := svc.ConfirmCaseAttachment(ctx, testAttachmentID)
			var nf *apierror.NotFoundError
			if !asNotFoundError(err, &nf) {
				t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
			}
		})
	})

	t.Run("DeleteCaseAttachment", func(t *testing.T) {
		t.Run("out of scope is denied, repo mutation never reached", func(t *testing.T) {
			repo := &stubCaseRepo{
				getCaseByID: scopedGetCaseByID(authTestProject),
				getCaseAttachmentByID: func(_ context.Context, id string) (domain.Attachment, error) {
					return domain.Attachment{ID: id, ReferenceID: testCaseID}, nil
				},
			}
			svc := NewCaseService(repo, actorUserRepo(t), nil, outOfScopeAccess)
			_, err := svc.DeleteCaseAttachment(ctx, domain.DeleteAttachmentRequest{AttachmentID: testAttachmentID})
			var nf *apierror.NotFoundError
			if !asNotFoundError(err, &nf) {
				t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
			}
		})

		t.Run("in scope succeeds, reaches repo", func(t *testing.T) {
			called := false
			repo := &stubCaseRepo{
				getCaseByID: scopedGetCaseByID(authTestProject),
				getCaseAttachmentByID: func(_ context.Context, id string) (domain.Attachment, error) {
					return domain.Attachment{ID: id, ReferenceID: testCaseID}, nil
				},
				deleteCaseAttachment: func(context.Context, string) error {
					called = true
					return nil
				},
			}
			svc := NewCaseService(repo, actorUserRepo(t), nil, inScopeAccess)
			if _, err := svc.DeleteCaseAttachment(ctx, domain.DeleteAttachmentRequest{AttachmentID: testAttachmentID}); err != nil {
				t.Fatalf("DeleteCaseAttachment: %v", err)
			}
			if !called {
				t.Fatal("expected repo.DeleteCaseAttachment to be called")
			}
		})
	})

	t.Run("GetAttachmentByID", func(t *testing.T) {
		t.Run("out of scope is denied", func(t *testing.T) {
			repo := &stubCaseRepo{
				getCaseByID: scopedGetCaseByID(authTestProject),
				getCaseAttachmentByID: func(_ context.Context, id string) (domain.Attachment, error) {
					return domain.Attachment{ID: id, ReferenceID: testCaseID}, nil
				},
			}
			svc := NewCaseService(repo, actorUserRepo(t), nil, outOfScopeAccess)
			_, err := svc.GetAttachmentByID(ctx, testAttachmentID)
			var nf *apierror.NotFoundError
			if !asNotFoundError(err, &nf) {
				t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
			}
		})

		t.Run("in scope succeeds", func(t *testing.T) {
			repo := &stubCaseRepo{
				getCaseByID: scopedGetCaseByID(authTestProject),
				getCaseAttachmentByID: func(_ context.Context, id string) (domain.Attachment, error) {
					return domain.Attachment{ID: id, ReferenceID: testCaseID}, nil
				},
			}
			svc := NewCaseService(repo, actorUserRepo(t), nil, inScopeAccess)
			if _, err := svc.GetAttachmentByID(ctx, testAttachmentID); err != nil {
				t.Fatalf("GetAttachmentByID: %v", err)
			}
		})
	})

	t.Run("UpdateAttachment", func(t *testing.T) {
		req := domain.UpdateAttachmentRequest{AttachmentID: testAttachmentID, ReferenceID: testCaseID, ReferenceType: domain.ReferenceTypeCase, Name: str("renamed.log")}

		t.Run("out of scope is denied, repo mutation never reached", func(t *testing.T) {
			repo := &stubCaseRepo{
				getCaseByID: scopedGetCaseByID(authTestProject),
				getCaseAttachmentByID: func(_ context.Context, id string) (domain.Attachment, error) {
					return domain.Attachment{ID: id, ReferenceID: testCaseID}, nil
				},
			}
			svc := NewCaseService(repo, actorUserRepo(t), nil, outOfScopeAccess)
			_, err := svc.UpdateAttachment(ctx, req)
			var nf *apierror.NotFoundError
			if !asNotFoundError(err, &nf) {
				t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
			}
		})

		t.Run("in scope succeeds, reaches repo", func(t *testing.T) {
			called := false
			repo := &stubCaseRepo{
				getCaseByID: scopedGetCaseByID(authTestProject),
				getCaseAttachmentByID: func(_ context.Context, id string) (domain.Attachment, error) {
					return domain.Attachment{ID: id, ReferenceID: testCaseID}, nil
				},
				updateAttachmentName: func(_ context.Context, id, name, updatedBy string) (time.Time, error) {
					called = true
					return time.Now(), nil
				},
			}
			svc := NewCaseService(repo, actorUserRepo(t), nil, inScopeAccess)
			if _, err := svc.UpdateAttachment(ctx, req); err != nil {
				t.Fatalf("UpdateAttachment: %v", err)
			}
			if !called {
				t.Fatal("expected repo.UpdateCaseAttachmentName to be called")
			}
		})
	})
}
