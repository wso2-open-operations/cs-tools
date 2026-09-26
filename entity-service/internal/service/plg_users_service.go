package service

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// PlgUserService searches the shared users table.
//
// Read-only, mirroring PlgUserRepository: PLG reads the CS team, it does not
// administer it.
type PlgUserService interface {
	SearchUsers(ctx context.Context, req domain.PlgSearchUsersRequest) (domain.PlgSearchUsersResponse, error)
}

type plgUserService struct {
	repo repository.PlgUserRepository
}

// NewPlgUserService wires a PlgUserService over its repository.
func NewPlgUserService(repo repository.PlgUserRepository) PlgUserService {
	return &plgUserService{repo: repo}
}

// Valid user_type values, mirroring the user_type_enum csm-portal declares.
//
// UPPERCASE, and four values: 'SYSTEM', 'INTERNAL', 'EXTERNAL' and
// 'NOT_AVAILABLE' — the last meaning a user carrying no role at all.
//
// Passing anything but 'INTERNAL' now returns nothing: the repository restricts
// every user query to internal staff regardless of what the caller asks for.
// The allow-list stays because a *mistyped* value should still be a 400 naming
// the field rather than a silently empty page.
//
// Validated here as well as by the column so a bad filter value is a 400 naming
// the field, not a 500 from a failed enum cast — the same reason
// entity-service keeps 46 of these maps beside its own enums.
var validUserType = map[string]bool{
	"SYSTEM":        true,
	"INTERNAL":      true,
	"EXTERNAL":      true,
	"NOT_AVAILABLE": true,
}

func (s *plgUserService) SearchUsers(ctx context.Context, req domain.PlgSearchUsersRequest) (domain.PlgSearchUsersResponse, error) {
	if err := plgValidateUUIDs("filters.ids", req.Filters.IDs); err != nil {
		return domain.PlgSearchUsersResponse{}, err
	}
	if err := validateEmails("filters.emails", req.Filters.Emails); err != nil {
		return domain.PlgSearchUsersResponse{}, err
	}
	if err := validateEnums("filters.userTypes", req.Filters.UserTypes, validUserType); err != nil {
		return domain.PlgSearchUsersResponse{}, err
	}
	plgNormalizePagination(&req.Pagination)

	users, total, err := s.repo.SearchUsers(ctx, req)
	if err != nil {
		return domain.PlgSearchUsersResponse{}, err
	}
	return domain.PlgSearchUsersResponse{
		Users:  users,
		Total:  total,
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
	}, nil
}
