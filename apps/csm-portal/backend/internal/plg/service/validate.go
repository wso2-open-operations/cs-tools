package service

import (
	"regexp"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
)

const (
	defaultPageLimit = 25
	maxPageLimit     = 100
)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// codeRe constrains task codes. They are the stable identity used to match a
// task across template revisions, so they must not carry punctuation or case
// variation that would silently create a "new" task on the next save.
var codeRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)

var nonCodeChars = regexp.MustCompile(`[^A-Z0-9]+`)

func validateUUID(field, value string) error {
	if !uuidRe.MatchString(value) {
		return apierror.Validation(field + " must be a UUID")
	}
	return nil
}

func validateEnum[T ~string](name string, value *T, valid map[T]bool) error {
	if value == nil {
		return nil
	}
	if !valid[*value] {
		return apierror.Validation("invalid " + name + ": " + string(*value))
	}
	return nil
}

func validateEnums[T ~string](name string, values []T, valid map[T]bool) error {
	for _, v := range values {
		if !valid[v] {
			return apierror.Validation("invalid " + name + ": " + string(v))
		}
	}
	return nil
}

func normalizePagination(p *domain.Pagination) {
	if p.Limit <= 0 {
		p.Limit = defaultPageLimit
	}
	if p.Limit > maxPageLimit {
		p.Limit = maxPageLimit
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
}

func validateSearchFilters(f *domain.OrganizationSearchFilters) error {
	if err := validateEnums("lifecycleStage", f.LifecycleStages, domain.ValidLifecycleStage); err != nil {
		return err
	}
	if err := validateEnums("subscriptionTier", f.SubscriptionTiers, domain.ValidSubscriptionTier); err != nil {
		return err
	}
	if f.RegisteredFrom != nil && f.RegisteredTo != nil && f.RegisteredTo.Before(*f.RegisteredFrom) {
		return apierror.Validation("registeredTo must not be earlier than registeredFrom")
	}
	return nil
}

// trimmedOrNil normalises optional free text: whitespace-only becomes nil, so it
// is never written as an empty string.
func trimmedOrNil(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

func parseDate(field string, value *string) error {
	if value == nil || *value == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", *value); err != nil {
		return apierror.Validation(field + " must be YYYY-MM-DD")
	}
	return nil
}

// deriveCode builds a stable code from a display name, so an engineer can type
// "Send the welcome email" without also inventing an identifier.
func deriveCode(name string) string {
	c := nonCodeChars.ReplaceAllString(strings.ToUpper(strings.TrimSpace(name)), "_")
	c = strings.Trim(c, "_")
	if len(c) > 64 {
		c = c[:64]
	}
	if c != "" && (c[0] < 'A' || c[0] > 'Z') {
		c = "T_" + c
	}
	return c
}

func ptr[T any](v T) *T { return &v }
