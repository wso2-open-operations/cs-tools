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

package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// PagingContactRepository reads and writes paging_contact (migration 0216):
// a person's paging-only phone number, and the last test call to it.
//
// Every write names the actor for the audit trigger in the same transaction
// (nameTheActor): the row has no updated_by for the trigger to fall back on,
// and a DELETE has no row left to ask.
type PagingContactRepository interface {
	// User is the person a number is for; a NotFoundError when there is no
	// such user.
	User(ctx context.Context, userID string) (PagingUser, error)
	// UserTeams is each user's teams, with the family the Team Schedule
	// gives each (CRE, SRE or SME). A user on no team has no entry.
	UserTeams(ctx context.Context, userIDs []string) (map[string][]PagingTeamRef, error)

	// Get is a user's stored number, or nil when none is stored.
	Get(ctx context.Context, userID string) (*PagingContactRow, error)
	// ByUserIDs is the stored numbers of the given users, by user id.
	ByUserIDs(ctx context.Context, userIDs []string) (map[string]PagingContactRow, error)
	// DialNumbersByEmails is, for the users with the given emails (compared
	// case-insensitively; an email held by two users yields both), the
	// number on their own profile ("user".phone) and their stored paging-only
	// number. A user with neither is left out.
	DialNumbersByEmails(ctx context.Context, emails []string) ([]PagingDialRow, error)
	// ProfilePhones is the callable number on each of the given users' own
	// profile ("user".phone, trimmed, matching CallablePhonePattern), by user
	// id. A user without one has no entry.
	ProfilePhones(ctx context.Context, userIDs []string) (map[string]string, error)
	// ProfileTests is the last test call of each of the given users' profile
	// number (paging_test_call, migration 0242), by user id.
	ProfileTests(ctx context.Context, userIDs []string) (map[string]PagingTestRow, error)

	// Upsert stores phone for the user. Changing the number clears the last
	// test, which was of another number; restating it keeps it.
	Upsert(ctx context.Context, actor, userID, phone string) (PagingContactRow, error)
	// Delete removes the user's number; a no-op when there is none.
	Delete(ctx context.Context, actor, userID string) error
	// MarkTestPending records a test call of the paging-only number as
	// requested at at, only when no test of the person's (of either number)
	// was requested after notBefore. False means a test was requested since,
	// or the number is gone.
	MarkTestPending(ctx context.Context, actor, userID string, at, notBefore time.Time) (PagingContactRow, bool, error)
	// MarkProfileTestPending records a test call of phone, the person's
	// profile number, as requested at at, with the same cooldown rule as
	// MarkTestPending. False means a test was requested since.
	MarkProfileTestPending(ctx context.Context, actor, userID, phone string, at, notBefore time.Time) (bool, error)
	// RecordTestResult stores a test call's outcome: on the pending test of
	// the profile number when that is the newest pending test, else on the
	// pending test of the paging-only number. False when neither is pending
	// (a late or repeated result is not recorded).
	RecordTestResult(ctx context.Context, actor, userID, status string, testedAt time.Time) (bool, error)
}

// PagingUser is the person a number belongs to.
type PagingUser struct {
	ID    string
	Email string
	Name  string
}

// PagingTeamRef is one of a person's teams.
type PagingTeamRef struct {
	TeamKey string
	Family  string
}

// CallablePhonePattern is the E.164 shape a profile number must have to be
// called: a +, then 8 to 15 digits, the first not 0. It matches the rule
// PATCH /users/me enforces on write. paging_contact's own CHECK is looser (7
// digits) and unchanged.
const CallablePhonePattern = `^\+[1-9][0-9]{7,14}$`

// PagingDialRow is one person's two numbers: the one on their own profile
// ("user".phone, migration 0141; "" when blank) and their paging-only number
// (nil when none is stored).
type PagingDialRow struct {
	UserID       string
	Email        string
	ProfilePhone string
	Paging       *PagingContactRow
}

// PagingTestRow is the last test call of a person's profile number
// (paging_test_call). Phone is the number that was rung.
type PagingTestRow struct {
	UserID         string
	Phone          string
	LastTestAt     time.Time
	LastTestStatus string
}

// PagingContactRow is one paging_contact row, with the user's email.
type PagingContactRow struct {
	UserID         string
	Email          string
	Name           string
	Phone          string
	SetBy          string
	SetAt          time.Time
	LastTestAt     *time.Time
	LastTestStatus *string
}

type pagingContactRepository struct{ db *pgxpool.Pool }

// NewPagingContactRepository constructs a PagingContactRepository over the pool.
func NewPagingContactRepository(db *pgxpool.Pool) PagingContactRepository {
	return &pagingContactRepository{db: db}
}

const pagingContactColumns = `
       pc.user_id::text, COALESCE(u.email, ''), ` + engineerName + `, pc.phone, pc.set_by,
       pc.set_at, pc.last_test_at, pc.last_test_status`

const pagingContactFrom = `
  FROM paging_contact pc
  JOIN "user" u ON u.id = pc.user_id`

func scanPagingContact(row pgx.Row) (PagingContactRow, error) {
	var c PagingContactRow
	err := row.Scan(&c.UserID, &c.Email, &c.Name, &c.Phone, &c.SetBy, &c.SetAt, &c.LastTestAt, &c.LastTestStatus)
	return c, err
}

func (r *pagingContactRepository) User(ctx context.Context, userID string) (PagingUser, error) {
	var u PagingUser
	err := r.db.QueryRow(ctx, `
		SELECT u.id::text, COALESCE(u.email, ''), `+engineerName+`
		  FROM "user" u WHERE u.id = $1::uuid`, userID).Scan(&u.ID, &u.Email, &u.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return PagingUser{}, &apierror.NotFoundError{Msg: "user not found"}
	}
	if err != nil {
		return PagingUser{}, fmt.Errorf("get paging user: %w", err)
	}
	return u, nil
}

func (r *pagingContactRepository) UserTeams(ctx context.Context, userIDs []string) (map[string][]PagingTeamRef, error) {
	out := map[string][]PagingTeamRef{}
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT m.user_id::text, t.key, `+teamFamilyExpr+`
		  FROM team_member m
		  JOIN team t ON t.id = m.team_id
		 WHERE m.user_id = ANY($1::uuid[]) AND t.key IS NOT NULL AND t.type IS NOT NULL
		 ORDER BY 1, 2`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("query paging user teams: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var t PagingTeamRef
		if err := rows.Scan(&id, &t.TeamKey, &t.Family); err != nil {
			return nil, fmt.Errorf("scan paging user team: %w", err)
		}
		out[id] = append(out[id], t)
	}
	return out, rows.Err()
}

func (r *pagingContactRepository) Get(ctx context.Context, userID string) (*PagingContactRow, error) {
	c, err := scanPagingContact(r.db.QueryRow(ctx, `SELECT `+pagingContactColumns+pagingContactFrom+`
		 WHERE pc.user_id = $1::uuid`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get paging contact: %w", err)
	}
	return &c, nil
}

func (r *pagingContactRepository) list(ctx context.Context, where string, arg any) ([]PagingContactRow, error) {
	rows, err := r.db.Query(ctx, `SELECT `+pagingContactColumns+pagingContactFrom+` WHERE `+where+` ORDER BY 2, 1`, arg)
	if err != nil {
		return nil, fmt.Errorf("query paging contacts: %w", err)
	}
	defer rows.Close()
	out := []PagingContactRow{}
	for rows.Next() {
		c, err := scanPagingContact(rows)
		if err != nil {
			return nil, fmt.Errorf("scan paging contact: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *pagingContactRepository) ByUserIDs(ctx context.Context, userIDs []string) (map[string]PagingContactRow, error) {
	out := map[string]PagingContactRow{}
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := r.list(ctx, `pc.user_id = ANY($1::uuid[])`, userIDs)
	if err != nil {
		return nil, err
	}
	for _, c := range rows {
		out[c.UserID] = c
	}
	return out, nil
}

func (r *pagingContactRepository) DialNumbersByEmails(ctx context.Context, emails []string) ([]PagingDialRow, error) {
	out := []PagingDialRow{}
	if len(emails) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT u.id::text, COALESCE(u.email, ''), COALESCE(btrim(u.phone), ''),
		       pc.phone, pc.set_by, pc.set_at, pc.last_test_at, pc.last_test_status
		  FROM "user" u
		  LEFT JOIN paging_contact pc ON pc.user_id = u.id
		 WHERE lower(u.email) = ANY(SELECT lower(e) FROM unnest($1::text[]) e)
		   AND (COALESCE(btrim(u.phone), '') <> '' OR pc.user_id IS NOT NULL)
		 ORDER BY 2, 1`, emails)
	if err != nil {
		return nil, fmt.Errorf("query paging dial numbers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var d PagingDialRow
		var phone, setBy *string
		var setAt *time.Time
		var c PagingContactRow
		if err := rows.Scan(&d.UserID, &d.Email, &d.ProfilePhone, &phone, &setBy, &setAt, &c.LastTestAt, &c.LastTestStatus); err != nil {
			return nil, fmt.Errorf("scan paging dial number: %w", err)
		}
		if phone != nil {
			c.UserID, c.Email, c.Phone = d.UserID, d.Email, *phone
			if setBy != nil {
				c.SetBy = *setBy
			}
			if setAt != nil {
				c.SetAt = *setAt
			}
			d.Paging = &c
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *pagingContactRepository) ProfilePhones(ctx context.Context, userIDs []string) (map[string]string, error) {
	out := map[string]string{}
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id::text, btrim(phone) FROM "user"
		 WHERE id = ANY($1::uuid[]) AND btrim(phone) ~ $2`, userIDs, CallablePhonePattern)
	if err != nil {
		return nil, fmt.Errorf("query profile phones: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, phone string
		if err := rows.Scan(&id, &phone); err != nil {
			return nil, fmt.Errorf("scan profile phone: %w", err)
		}
		out[id] = phone
	}
	return out, rows.Err()
}

func (r *pagingContactRepository) ProfileTests(ctx context.Context, userIDs []string) (map[string]PagingTestRow, error) {
	out := map[string]PagingTestRow{}
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT user_id::text, phone, last_test_on, last_test_status
		  FROM paging_test_call WHERE user_id = ANY($1::uuid[])`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("query profile test calls: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var t PagingTestRow
		if err := rows.Scan(&t.UserID, &t.Phone, &t.LastTestAt, &t.LastTestStatus); err != nil {
			return nil, fmt.Errorf("scan profile test call: %w", err)
		}
		out[t.UserID] = t
	}
	return out, rows.Err()
}

// inTx runs fn in one transaction with the actor named for the audit trigger.
func (r *pagingContactRepository) inTx(ctx context.Context, actor string, fn func(pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin paging contact change: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := nameTheActor(ctx, tx, actor); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit paging contact change: %w", err)
	}
	return nil
}

func (r *pagingContactRepository) Upsert(ctx context.Context, actor, userID, phone string) (PagingContactRow, error) {
	var c PagingContactRow
	err := r.inTx(ctx, actor, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO paging_contact (user_id, phone, set_by, set_at)
			VALUES ($1::uuid, $2, $3, now())
			ON CONFLICT (user_id) DO UPDATE
			   SET phone = EXCLUDED.phone, set_by = EXCLUDED.set_by, set_at = now(),
			       last_test_at = CASE WHEN paging_contact.phone = EXCLUDED.phone THEN paging_contact.last_test_at END,
			       last_test_status = CASE WHEN paging_contact.phone = EXCLUDED.phone THEN paging_contact.last_test_status END`,
			userID, phone, actor); err != nil {
			return fmt.Errorf("store paging contact: %w", err)
		}
		var err error
		c, err = scanPagingContact(tx.QueryRow(ctx, `SELECT `+pagingContactColumns+pagingContactFrom+`
			 WHERE pc.user_id = $1::uuid`, userID))
		if err != nil {
			return fmt.Errorf("read stored paging contact: %w", err)
		}
		return nil
	})
	return c, err
}

func (r *pagingContactRepository) Delete(ctx context.Context, actor, userID string) error {
	return r.inTx(ctx, actor, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM paging_contact WHERE user_id = $1::uuid`, userID); err != nil {
			return fmt.Errorf("delete paging contact: %w", err)
		}
		return nil
	})
}

func (r *pagingContactRepository) MarkTestPending(ctx context.Context, actor, userID string, at, notBefore time.Time) (PagingContactRow, bool, error) {
	var c PagingContactRow
	marked := false
	err := r.inTx(ctx, actor, func(tx pgx.Tx) error {
		// One statement decides, so two requests at once cannot both pass
		// the cooldown.
		tag, err := tx.Exec(ctx, `
			UPDATE paging_contact
			   SET last_test_status = 'pending', last_test_at = $3
			 WHERE user_id = $1::uuid
			   AND (last_test_at IS NULL OR last_test_at <= $2)
			   AND NOT EXISTS (SELECT 1 FROM paging_test_call t
			                    WHERE t.user_id = $1::uuid AND t.last_test_on > $2)`, userID, notBefore, at)
		if err != nil {
			return fmt.Errorf("mark paging test pending: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		marked = true
		c, err = scanPagingContact(tx.QueryRow(ctx, `SELECT `+pagingContactColumns+pagingContactFrom+`
			 WHERE pc.user_id = $1::uuid`, userID))
		if err != nil {
			return fmt.Errorf("read paging contact: %w", err)
		}
		return nil
	})
	return c, marked, err
}

func (r *pagingContactRepository) MarkProfileTestPending(ctx context.Context, actor, userID, phone string, at, notBefore time.Time) (bool, error) {
	marked := false
	err := r.inTx(ctx, actor, func(tx pgx.Tx) error {
		// One statement decides, so two requests at once cannot both pass
		// the cooldown; a pending test of the paging-only number counts too.
		tag, err := tx.Exec(ctx, `
			INSERT INTO paging_test_call AS t (user_id, phone, requested_by, last_test_on, last_test_status)
			SELECT $1::uuid, $2::text, $3::text, $4::timestamptz, 'pending'
			 WHERE NOT EXISTS (SELECT 1 FROM paging_contact pc
			                    WHERE pc.user_id = $1::uuid AND pc.last_test_at > $5)
			ON CONFLICT (user_id) DO UPDATE
			   SET phone = EXCLUDED.phone, requested_by = EXCLUDED.requested_by,
			       last_test_on = EXCLUDED.last_test_on, last_test_status = 'pending'
			 WHERE t.last_test_on <= $5`, userID, phone, actor, at, notBefore)
		if err != nil {
			return fmt.Errorf("mark profile test pending: %w", err)
		}
		marked = tag.RowsAffected() > 0
		return nil
	})
	return marked, err
}

func (r *pagingContactRepository) RecordTestResult(ctx context.Context, actor, userID, status string, testedAt time.Time) (bool, error) {
	found := false
	err := r.inTx(ctx, actor, func(tx pgx.Tx) error {
		// The result is of the newest pending test: the profile number's when
		// no pending test of the paging-only number is newer.
		tag, err := tx.Exec(ctx, `
			UPDATE paging_test_call t SET last_test_status = $2, last_test_on = $3
			 WHERE t.user_id = $1::uuid AND t.last_test_status = 'pending'
			   AND NOT EXISTS (SELECT 1 FROM paging_contact pc
			                    WHERE pc.user_id = $1::uuid AND pc.last_test_status = 'pending'
			                      AND pc.last_test_at > t.last_test_on)`, userID, status, testedAt)
		if err != nil {
			return fmt.Errorf("record profile test result: %w", err)
		}
		if tag.RowsAffected() > 0 {
			found = true
			return nil
		}
		// Only a pending test: a late or repeated result -- of a profile
		// test already settled, or of this number's own -- must not
		// overwrite the paging-only number's last result.
		tag, err = tx.Exec(ctx, `
			UPDATE paging_contact SET last_test_status = $2, last_test_at = $3
			 WHERE user_id = $1::uuid AND last_test_status = 'pending'`, userID, status, testedAt)
		if err != nil {
			return fmt.Errorf("record paging test result: %w", err)
		}
		found = tag.RowsAffected() > 0
		return nil
	})
	return found, err
}
