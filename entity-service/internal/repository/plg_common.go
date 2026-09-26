// Package repository implements every PostgreSQL query the PLG slice issues.
// All SQL uses parameterised queries — never string-interpolated user input.
//
// Two conventions run through all of it:
//
//   - A UserRef is three columns — id, email, name. The id identifies; the
//     other two are for display.
//   - Id filters compare `<column>::TEXT = ANY($n::TEXT[])`, casting the column
//     rather than the parameter. Every id filter here reads the same way,
//     owner included.
//   - actorWrite names `users` rather than `plg_cs_user`.
package repository

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// argBuilder accumulates positional query arguments so filter clauses can be
// composed without ever interpolating a caller-supplied value into the SQL.
type argBuilder struct {
	args []any
}

// add appends v and returns its positional placeholder ("$3").
func (b *argBuilder) add(v any) string {
	b.args = append(b.args, v)
	return "$" + strconv.Itoa(len(b.args))
}

// list returns the accumulated arguments.
func (b *argBuilder) list() []any { return b.args }

// whereClause joins conditions into a WHERE fragment, or "" when there are none.
func whereClause(conds []string) string {
	if len(conds) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conds, " AND ")
}

// userRef assembles a CS engineer reference from the three nullable columns a
// LEFT JOIN on users produces. Nil when the row carried no owner.
//
// The id is what identifies; the email and name are carried for display and are
// never matched on.
//
// Name falls back to the email when the join produced no name, which can only
// happen if first_name and last_name are both blank. Both are NOT NULL, so this
// is defensive rather than expected — but a blank label on screen is worse than
// an address.
func userRef(id, email, name *string) *domain.UserRef {
	if id == nil || *id == "" {
		return nil
	}
	ref := &domain.UserRef{ID: *id}
	if email != nil {
		ref.Email = *email
	}
	switch {
	case name != nil && *name != "":
		ref.Name = *name
	default:
		ref.Name = ref.Email
	}
	return ref
}

// productRef assembles a PlgProductRef from three non-null columns.
func productRef(id, code, name string) domain.PlgProductRef {
	return domain.PlgProductRef{ID: id, Code: code, Name: name}
}

// stringsOf converts a slice of string-kinded enums to plain strings, so they
// can be bound as a TEXT[] and compared with an explicit cast.
func stringsOf[T ~string](in []T) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, string(v))
	}
	return out
}

// enumArg converts a *T enum pointer into an *string pgx can bind, so a nil
// pointer becomes SQL NULL and COALESCE leaves the column unchanged.
func enumArg[T ~string](v *T) *string {
	if v == nil {
		return nil
	}
	s := string(*v)
	return &s
}

// uuidArg converts a possibly-empty id string into something pgx binds as a
// nullable UUID: "" becomes SQL NULL rather than a cast error.
//
// Needed in every write that records an actor. Wrapping the parameter in NULLIF
// inline does not work against a UUID column: an empty string is not a UUID, so
// Postgres fails on the cast before NULLIF ever runs. Converting to a nil
// pointer on this side binds a real SQL NULL instead.
func uuidArg(id string) *string {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	return &id
}

// isUniqueViolation lives in github_mutation_repo.go — same package, identical
// implementation. PLG had its own copy until upstream added one; the duplicate
// went rather than the two being kept in step.

// isConstraintViolation reports whether err is a foreign-key or check violation.
//
// Both matter here because the schema does real work with them: the composite FK
// on plg_playbook refuses a playbook at a stage that carries none, and the CHECK
// constraints refuse a mistyped task value. Surfacing them as 400 rather than
// 500 turns a stack trace into an explanation.
func isConstraintViolation(err error) bool {
	return isForeignKeyViolation(err) || isCheckViolation(err)
}

// isForeignKeyViolation reports a 23503 — a row pointing at something that does
// not exist. Distinct from a check violation because the two mean different
// things to a caller, and the same statement can raise either.
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// isCheckViolation reports a 23514, including the triggers that raise it
// deliberately.
func isCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514"
}

// actorWrite wraps an error from a statement that records who performed an
// action.
//
// Every such column is a foreign key into users, so an actor id the table does
// not hold fails the write. That is a caller problem, not a server fault:
// without this it surfaces as a 500 saying "internal server error", which tells
// an engineer whose account has not been provisioned nothing at all.
//
// It rarely fires, because the BFF resolves the caller's email to an id before
// any write and rejects an unresolvable caller up front. It stays because the
// resolution and the write are not in the same transaction: an engineer
// deactivated between the two would reach
// here.
func actorWrite(err error, actorID, operation string) error {
	if isForeignKeyViolation(err) {
		return &apierror.ValidationError{Msg: "unknown CS user: " + actorID +
			" — the portal records who performed each action, so the caller must exist in users"}
	}
	// A check violation here is one of the schema's deliberate refusals — the
	// forward-only stage trigger, or the constraint that a history row explain
	// itself. Those raise messages written for a person to read, so the message
	// is passed through rather than replaced.
	//
	// Without this the trigger's sentence became a 500 saying "internal server
	// error". The service layer catches the same rule earlier and answers 400,
	// but only when the caller sent expectedStage — so a plain PATCH that moved
	// a pairing backwards was refused correctly by the database and reported as
	// though the server had broken.
	if msg, ok := checkViolationMessage(err); ok {
		return &apierror.ValidationError{Msg: msg}
	}
	return fmt.Errorf("%s: %w", operation, err)
}

// checkViolationMessage returns the message a CHECK or a RAISE carried, when
// the error was one.
func checkViolationMessage(err error) (string, bool) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		return "", false
	}
	// A table constraint reports its own name and nothing a caller can act on;
	// a RAISE from a trigger reports a sentence. Prefer the sentence, and fall
	// back to naming the rule that was broken.
	if pgErr.ConstraintName != "" && pgErr.Message == "" {
		return "the write broke constraint " + pgErr.ConstraintName, true
	}
	return pgErr.Message, true
}

// toLower is strings.ToLower, named locally so the email comparison in
// plg_users_repo.go reads as a decision rather than an incidental call.
