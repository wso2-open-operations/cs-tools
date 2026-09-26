// Package service holds the slice's validation and query orchestration.
//
// WHAT IS AND IS NOT HERE. This layer validates input and normalises
// pagination — the two things entity-service's own service layer does for every
// entity, with 46 enum maps and 300 validation calls already following exactly
// this pattern. PLG's *lifecycle* rules are NOT here: they stayed in the BFF,
// which is the split plg-docs/ENTITY-SERVICE-CONTRACT.md sets out. So there is no
// "can this stage move there" and no "must an organisation keep an owner" in
// this package; there is only "is that a UUID" and "is that a known enum".
package service

import (
	"regexp"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const (
	defaultPageLimit = 25
	maxPageLimit     = 100
)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// validateUUID rejects a non-UUID before it reaches Postgres.
//
// Every actor, owner and author is a UUID, so this is what stands between a
// malformed id and a Postgres cast error — the caller gets a 400 naming the
// field instead of a 500 naming a type.
func validateUUID(field, value string) error {
	if !uuidRe.MatchString(value) {
		return &apierror.ValidationError{Msg: field + " must be a UUID"}
	}
	return nil
}

// plgValidateUUIDs applies validateUUID across a slice, naming the index that
// failed so a caller sending twenty ids learns which one is wrong.
func plgValidateUUIDs(field string, values []string) error {
	for i, v := range values {
		if err := validateUUID(field, v); err != nil {
			return &apierror.ValidationError{
				Msg: field + "[" + plgItoa(i) + "] must be a UUID",
			}
		}
	}
	return nil
}

func validateEnums[T ~string](name string, values []T, valid map[T]bool) error {
	for _, v := range values {
		if !valid[v] {
			return &apierror.ValidationError{Msg: "invalid " + name + ": " + string(v)}
		}
	}
	return nil
}

// plgNormalizePagination clamps the page size, matching entity-service's own
// plgNormalizePagination: limit defaults to 25 and is capped at 100.
//
// Clamps silently rather than erroring, matching entity-service's own. Worth
// knowing when reading a response: a request for 10000 returns 100 and says so
// only through the `limit` it echoes back.
// plgNormalizePagination CLAMPS an oversized limit; entity-service's own
// normalizePagination REJECTS one with a 400. Both are defensible and they are
// not interchangeable, so PLG keeps its own rather than adopting upstream's and
// silently changing what an over-large limit does to a PLG caller.
//
// The prefix is not decoration: without it this is a duplicate declaration in
// the merged package, which is how the difference was noticed at all.
func plgNormalizePagination(p *domain.Pagination) {
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

// validateEmails rejects anything that cannot be an address.
//
// Deliberately shallow: the only caller is the BFF resolving X-PLG-User, and a
// value that is not an address simply will not match a row. This exists to keep
// obvious junk out of a query, not to validate deliverability.
func validateEmails(field string, values []string) error {
	for i, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || !strings.Contains(v, "@") {
			return &apierror.ValidationError{
				Msg: field + "[" + plgItoa(i) + "] must be an email address",
			}
		}
	}
	return nil
}

func plgItoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}

// invalidEnum and invalidField are the two shapes of validation failure this
// package raises, named so call sites read as decisions rather than as
// struct literals.
func invalidEnum(name, value string) error {
	return &apierror.ValidationError{Msg: "invalid " + name + ": " + value}
}

func invalidField(msg string) error {
	return &apierror.ValidationError{Msg: msg}
}

// parseDate accepts YYYY-MM-DD, or nothing at all.
func parseDate(field string, value *string) error {
	if value == nil || *value == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", *value); err != nil {
		return &apierror.ValidationError{Msg: field + " must be YYYY-MM-DD"}
	}
	return nil
}
