-- Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
--
-- WSO2 LLC. licenses this file to you under the Apache License,
-- Version 2.0 (the "License"); you may not use this file except
-- in compliance with the License.
-- You may obtain a copy of the License at
--
-- http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing,
-- software distributed under the License is distributed on an
-- "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
-- KIND, either express or implied.  See the License for the
-- specific language governing permissions and limitations
-- under the License.

-- PLG Customer Success Portal — core schema.
--
-- This migration owns every plg_* table, enum, view and trigger. It attaches to
-- csm-portal's own `"user"` table (000001_users_table) rather than defining a
-- users table of its own: the twelve foreign keys below point at `"user"(id)`,
-- a UUID, matching 000001 exactly so nothing casts. The name is quoted because
-- the table is `user`, singular, which is a reserved word.
--
-- CS engineers are keyed by surrogate id, never by email. Email is PII, and
-- keying on it would copy one engineer's address into a dozen tables — so an
-- address change means rewriting all of them, and exporting any one table leaks
-- the team's addresses. Email lives in exactly one place, `"user".email`, and
-- reaches the API through the views for display only. It is never a key.
--
-- THE LIFECYCLE HAS TWO AXES, and keeping them apart is the central idea here:
--
--   stage   where the pairing has got to. Six values, and a pairing may only
--           move FORWARD along the first five, or out to ABANDONED, which is
--           terminal.
--   health  how it is going, wherever it is. HEALTHY or AT_RISK, set by an
--           engineer, changeable in either direction at any stage.
--
-- Being in trouble is not a place in the progression — a customer at risk has
-- not lost the stage they reached — so health is its own axis rather than a
-- stage. For the same reason the commercial facts (what a customer pays) live
-- on the subscription tier, and an outcome like a disqualified registration is
-- recorded by a playbook that can say WHY, not by a stage that cannot.
--
-- Playbooks split along the same seam, into three kinds: a PROGRESSIVE one
-- carries a pairing to the next stage, a RECOVERY one restores its health
-- without moving it, and a SUSTAINING one keeps a healthy pairing steady where
-- it is. Health decides which kinds are on offer.
--
-- The reasoning behind each table is in plg-docs/ER.md.


-- ---------------------------------------------------------------------------
-- Enums
-- ---------------------------------------------------------------------------

-- The six stages.
--
-- Declared in movement order, which is load-bearing: a pairing may only move
-- FORWARD along the first five, or out to ABANDONED from anywhere. Nothing
-- moves backwards and nothing leaves ABANDONED. See plg_check_stage_move().
--
-- Three things that look like stages deliberately are not:
--
--   being at risk    a pairing can be at risk at ANY stage, so making it a
--                    stage would cost it the stage it was actually at. It is a
--                    second axis — see plg_health_enum.
--   what they pay    the subscription tier records that. COMMERCIAL is the one
--                    stage that means money is involved; the tier says how.
--   a disqualified   an outcome, not a place. It is a progressive playbook at
--   registration     REGISTRATION that ends the pairing at ABANDONED — and
--                    unlike a stage, a playbook can record WHY, through the
--                    checklist reasons plg_run_task_reason_v already reports.
DO $$ BEGIN
    CREATE TYPE plg_lifecycle_stage_enum AS ENUM (
    'REGISTRATION',
    'PLG_CS_ELIGIBLE',
    'FIRST_VALUE_ACHIEVED',
    'ACTIVATED',
    'COMMERCIAL',
    'ABANDONED'
);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- How a pairing is doing, wherever it has got to.
--
-- The second axis, and the reason the stage list could shrink. A pairing can
-- fall out of health at any stage and recover without losing its place, which
-- "AT_RISK as a stage" could not express: a customer at risk had, by
-- definition, no stage.
--
-- Set by an engineer, not derived. Defaults to HEALTHY.
DO $$ BEGIN
    CREATE TYPE plg_health_enum AS ENUM ('HEALTHY', 'AT_RISK');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- What a playbook is for. The lifecycle seen from the playbook's side.
--
--   PROGRESSIVE  carries a pairing forward, to the next stage
--   RECOVERY     carries it from AT_RISK back to HEALTHY, in place
--   SUSTAINING   keeps a healthy pairing healthy, without moving it on
--
-- SUSTAINING is the kind that was missing. With two kinds the model said every
-- piece of work either advances a customer or rescues one, so the work that
-- holds a good account steady -- the check-in, the quarterly review, the
-- upgrade nobody is asking for yet -- had to be dressed as progression and
-- pinned to a stage it was not really trying to leave. Naming it separately
-- also means a stage can carry holding work without that work counting as a
-- route out of the stage.
--
-- Which kinds a pairing is offered is a function of health, and that function
-- is plg_applicable_playbook_types below. It is not stored on the pairing: a
-- stored copy is a second opinion waiting to disagree with the first.
DO $$ BEGIN
    CREATE TYPE plg_playbook_type_enum AS ENUM ('PROGRESSIVE', 'RECOVERY', 'SUSTAINING');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Which kinds of playbook a pairing in this health state may be offered.
--
-- A healthy pairing gets both the work that moves it on and the work that keeps
-- it where it is; an at-risk one gets neither, because neither is the job in
-- front of the engineer. This is the whole of the rule, and every reader of it
-- -- the pairing view, the work queue, the available-playbook list -- calls
-- here rather than restating it. Three copies of a CASE are three chances to
-- disagree.
--
-- IMMUTABLE so the planner can inline it into the EXISTS and the lateral below
-- rather than calling it per row.
CREATE OR REPLACE FUNCTION plg_applicable_playbook_types(h plg_health_enum)
RETURNS plg_playbook_type_enum[]
LANGUAGE SQL IMMUTABLE PARALLEL SAFE AS $$
    SELECT CASE h
               WHEN 'AT_RISK' THEN ARRAY['RECOVERY']
               ELSE ARRAY['PROGRESSIVE', 'SUSTAINING']
           END::plg_playbook_type_enum[]
$$;

-- What a playbook task holds. Tasks stopped being checkboxes because most of
-- them never were: "Company profile" wants a paragraph, "Fit score" wants a
-- number, and only a bookend genuinely wants a tick.
-- CHECKLIST and SINGLE_SELECT are the two that offer a fixed list of answers;
-- they differ only in how many may be chosen. Both require `options`, and both
-- report through plg_run_task_reason_v, so "why was this disqualified" and
-- "which plan did they pick" are the same query.
DO $$ BEGIN
    CREATE TYPE plg_task_value_type_enum AS ENUM (
    'BOOLEAN', 'STRING', 'NUMBER', 'CHECKLIST', 'SINGLE_SELECT');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Recorded by hand — there is no subscription feed.
--
-- The order is the commercial progression: a customer arrives FREE or starts a
-- TRIAL, may be granted a TRIAL_EXTENDED period, and converts to PAYG.
--
-- ENTERPRISE is gone. It described how a contract was signed rather than what
-- the customer is paying, and nothing in the portal ever branched on it — an
-- enterprise agreement is a PAYG relationship with a different piece of paper
-- behind it. TRIAL_EXTENDED replaced it because that IS a state the portal acts
-- on: it has its own end date, and a customer sitting in it is one whose trial
-- has already been rescued once.
DO $$ BEGIN
    CREATE TYPE plg_subscription_tier_enum AS ENUM ('FREE', 'TRIAL', 'TRIAL_EXTENDED', 'PAYG');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Which of a pairing's two trial dates is the one currently running.
--
-- The tier decides, and this function is the only place that says so — the same
-- treatment plg_applicable_playbook_types gives the health/playbook rule, and
-- for the same reason: the portal asks "when does this customer's clock run
-- out" in the dashboard, in the UI and in any report anyone writes later, and
-- three copies of a CASE is three chances to disagree.
--
-- NULL for FREE and PAYG, and that is the answer rather than missing data:
-- neither tier has a period that ends.
CREATE OR REPLACE FUNCTION plg_current_period_end_date(
    tier plg_subscription_tier_enum, trial DATE, extended DATE)
RETURNS DATE
LANGUAGE SQL IMMUTABLE PARALLEL SAFE AS $$
    SELECT CASE tier
               WHEN 'TRIAL'          THEN trial
               WHEN 'TRIAL_EXTENDED' THEN extended
           END
$$;

-- Shared updated_at trigger. Prefixed, so it does not collide with any
-- same-purpose function entity-service defines for its own tables.
CREATE OR REPLACE FUNCTION plg_set_updated_at() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

-- ---------------------------------------------------------------------------
-- Reference data
-- ---------------------------------------------------------------------------

-- Prefixed plg_product, not `products`: entity-service already has a `products`
-- table holding WSO2 products, and this is a different thing — the five cloud
-- platforms a PLG pairing can be on. Only `"user"` is deliberately shared.
CREATE TABLE IF NOT EXISTS plg_product (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code          TEXT        NOT NULL UNIQUE,
    name          TEXT        NOT NULL,
    display_order INT         NOT NULL DEFAULT 0,
    active        BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE TRIGGER trg_plg_product_updated_at BEFORE UPDATE ON plg_product
    FOR EACH ROW EXECUTE FUNCTION plg_set_updated_at();

-- The platform name arrives already normalised to one of these codes: the
-- upstream webhook handler maps the source's vocabulary before we see it.
INSERT INTO plg_product (code, name, display_order) VALUES
    ('IAM',                  'Identity & Access Management', 1),
    ('API_PLATFORM',         'API Platform',                 2),
    ('INTEGRATION_PLATFORM', 'Integration Platform',         3),
    ('AGENT_PLATFORM',       'Agent Platform',               4),
    ('ENG_PLATFORM',         'Engineering Platform',         5)
ON CONFLICT (code) DO NOTHING;

-- The stage catalogue. `name` exists because no string transform turns PAYG
-- into "PayG" — you get "Payg". The label has to be authored somewhere, and the
-- database means the webapp and any SQL report read the same one.
-- The stage catalogue. `name` exists because no string transform turns
-- PLG_CS_ELIGIBLE into "PLG CS Eligible" — the label has to be authored
-- somewhere, and the database means the webapp and any SQL report read the same
-- one.
--
-- display_order is no longer only a display concern. It defines what "forward"
-- means, and plg_check_stage_move() compares it, so reordering these rows
-- changes which moves are legal. ABANDONED sits outside the progression at 99
-- deliberately: it is reachable from anywhere rather than being the step after
-- COMMERCIAL.
CREATE TABLE IF NOT EXISTS plg_lifecycle_stage (
    stage         plg_lifecycle_stage_enum PRIMARY KEY,
    name          TEXT NOT NULL,
    display_order INT  NOT NULL UNIQUE,
    description   TEXT
);

INSERT INTO plg_lifecycle_stage (stage, name, display_order, description) VALUES
    ('REGISTRATION',         'Registration',         1,  'A registration has arrived and nobody has qualified it yet.'),
    ('PLG_CS_ELIGIBLE',      'PLG CS Eligible',      2,  'Qualified: a real organisation with a plausible use case, worth the team''s time.'),
    ('FIRST_VALUE_ACHIEVED', 'First Value Achieved', 3,  'The product did something useful for them for the first time.'),
    ('ACTIVATED',            'Activated',            4,  'Using the product repeatedly and under their own steam.'),
    ('COMMERCIAL',           'Commercial',           5,  'Paying. Which plan is the subscription tier''s business, not the lifecycle''s.'),
    ('ABANDONED',            'Abandoned',            99, 'Gone. Terminal — reachable from any stage, and nothing leaves it.')
ON CONFLICT (stage) DO NOTHING;

-- PLG_PLAYBOOK_STAGE_PATH IS GONE.
--
-- It mapped which stages carried playbooks and where each playbook was heading
-- — seven rows describing a branching graph. Both jobs disappeared:
--
--   "which stages carry playbooks"  every stage now carries both kinds, so the
--                                   question has one answer and needs no table.
--   "where is this playbook going"  a progressive playbook goes to the next
--                                   stage and a risk one goes nowhere, so the
--                                   destination is implied by the type.
--
-- What is left — the legal moves — is not a lookup table either. It is one
-- comparison over display_order, enforced by the trigger below.

-- ---------------------------------------------------------------------------
-- Registration intake
-- ---------------------------------------------------------------------------

-- The human who registered — a customer, not a CS engineer.
--
-- Keyed on a surrogate, with email as a UNIQUE attribute rather than the key —
-- the same shape `"user"` has. Email is on every payload the source sends —
-- including the sparse second-registration one — which is what makes the second
-- organisation resolvable without a mapping table.
CREATE TABLE IF NOT EXISTS plg_person (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email      TEXT        NOT NULL UNIQUE,
    first_name TEXT,
    last_name  TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE TRIGGER trg_plg_person_updated_at BEFORE UPDATE ON plg_person
    FOR EACH ROW EXECUTE FUNCTION plg_set_updated_at();

-- One Asgardeo organisation.
--
-- organization_name is globally unique, which does two jobs: it makes a
-- redelivered webhook event idempotent (the source's record id is absent on second
-- registrations, so there is nothing else to deduplicate on), and it is the key
-- passed to the runtime product-analytics API.
CREATE TABLE IF NOT EXISTS plg_organization (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_name        TEXT        NOT NULL UNIQUE,
    created_on               TIMESTAMPTZ NOT NULL,

    -- INSERT-ONLY. Never listed in the upsert's DO UPDATE SET, so a colleague
    -- adding a second platform to an existing organisation cannot overwrite the
    -- record of who first registered it. Per-platform attribution is not kept.
    registered_user          UUID        NOT NULL REFERENCES plg_person (id) ON DELETE RESTRICT,

    -- One CS owner handles every pairing for the organisation.
    --
    -- RESTRICT, not SET NULL: the work queue reads ownership to decide whose
    -- queue a pairing is in, so an ownerless organisation is a hole — its
    -- pairings stay in the queue but appear in nobody's view. Offboarding is
    -- what "user".is_active is for.
    plg_cs_owner             UUID        REFERENCES "user" (id) ON DELETE RESTRICT,

    -- Absent on a sparse second registration, and resolved at read time from
    -- the registrant's other organisations rather than copied forward — see
    -- plg_organization_v.
    country_name             TEXT,
    company_name_from_domain TEXT,

    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Moesif's identifier for the company. Unique but nullable: a record that
    -- arrives without one still lands, keyed on organization_name as always.
    moesif_company_id        TEXT,

    -- Named explicitly rather than left to Postgres: an inline UNIQUE would be
    -- called plg_organization_moesif_company_id_key, and a constraint name is
    -- what a violation reports and what a later migration has to reference.
    CONSTRAINT uq_plg_organization_moesif_company UNIQUE (moesif_company_id)
);

CREATE INDEX IF NOT EXISTS idx_plg_organization_registrant ON plg_organization (registered_user);
CREATE INDEX IF NOT EXISTS idx_plg_organization_owner      ON plg_organization (plg_cs_owner);
CREATE INDEX IF NOT EXISTS idx_plg_organization_created    ON plg_organization (created_on DESC);

CREATE OR REPLACE TRIGGER trg_plg_organization_updated_at BEFORE UPDATE ON plg_organization
    FOR EACH ROW EXECUTE FUNCTION plg_set_updated_at();

-- The organisation+platform pairing — the unit of work, and the registration.
--
-- One webhook event carrying a platform produces exactly one row. An event with
-- no platform creates the organisation and no pairing: the organisation exists
-- with zero platforms until we learn one, rather than the registration being
-- rejected.
CREATE TABLE IF NOT EXISTS plg_org_platform (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id       UUID NOT NULL REFERENCES plg_organization (id) ON DELETE CASCADE,
    product_id            UUID NOT NULL REFERENCES plg_product (id)      ON DELETE RESTRICT,

    -- From the payload.
    registered_on         TIMESTAMPTZ NOT NULL,

    -- The lifecycle. Belongs here rather than on the organisation, which is the
    -- whole point: two products under one customer can sit in different stages.
    lifecycle_stage       plg_lifecycle_stage_enum NOT NULL DEFAULT 'REGISTRATION'
                              REFERENCES plg_lifecycle_stage (stage),
    stage_entered_on      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    stage_updated_by      UUID REFERENCES "user" (id) ON DELETE SET NULL,

    -- The second axis. Mirrors the three stage columns above deliberately: a
    -- health change is the same kind of event as a stage change — an engineer
    -- recording a judgement — and the product tab shows both on one timeline.
    --
    -- Defaults to HEALTHY. A pairing is fine until somebody says otherwise;
    -- there is no third "unknown" value, because an engineer who has not looked
    -- is not the same as a customer in trouble and the queue should not treat
    -- them alike.
    health_state          plg_health_enum NOT NULL DEFAULT 'HEALTHY',
    health_entered_on     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    health_updated_by     UUID REFERENCES "user" (id) ON DELETE SET NULL,

    -- "New" is the absence of this timestamp. Nothing stores a flag.
    acknowledged_on       TIMESTAMPTZ,
    acknowledged_by       UUID REFERENCES "user" (id) ON DELETE SET NULL,

    -- The third tracked axis.
    --
    -- Filled in by the CS team, not by any source — there is no subscription
    -- feed. It gets the same three columns the stage and health do, for the same
    -- reason: a tier change is a judgement somebody made, and "who said this
    -- customer went PayG, and when" is a question that gets asked.
    --
    -- Unlike health, a reason is REQUIRED on a tier change (enforced in the
    -- BFF). Money moving is the kind of fact a colleague will want explained,
    -- and unlike a stage move it cannot be inferred from anything else in the
    -- record.
    subscription_tier        plg_subscription_tier_enum,
    subscription_entered_on  TIMESTAMPTZ,
    subscription_updated_by  UUID REFERENCES "user" (id) ON DELETE SET NULL,

    -- One date per dated tier, not one shared date.
    --
    -- Two columns rather than a single "current period ends" because they
    -- answer different questions and both get asked: trial_end_date is when the
    -- original trial ran out, trial_extended_date is when the extension does.
    -- Collapsing them would lose the first the moment an extension was granted,
    -- and "how long was the original trial" is exactly what someone reviewing a
    -- stalled conversion wants to know.
    --
    -- Deliberately NOT constrained to the tier. An engineer sets the date and
    -- the tier in either order, and a pairing that converts to PAYG keeps the
    -- trial dates it had — they are a record of what happened, not live state.
    -- Which date is CURRENT is a function of subscription_tier; see
    -- plg_current_period_end_date above, which is where that rule lives.
    trial_end_date        DATE,
    trial_extended_date   DATE,

    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_plg_org_platform UNIQUE (organization_id, product_id)
);

CREATE INDEX IF NOT EXISTS idx_plg_org_platform_stage       ON plg_org_platform (lifecycle_stage);
-- The work queue filters on the pair, and offers playbooks by health.
CREATE INDEX IF NOT EXISTS idx_plg_org_platform_health      ON plg_org_platform (health_state, lifecycle_stage);
CREATE INDEX IF NOT EXISTS idx_plg_org_platform_product     ON plg_org_platform (product_id, lifecycle_stage);
CREATE INDEX IF NOT EXISTS idx_plg_org_platform_registered  ON plg_org_platform (registered_on DESC);
-- Drives the "New registrations" tile and panel.
CREATE INDEX IF NOT EXISTS idx_plg_org_platform_unack       ON plg_org_platform (registered_on DESC)
    WHERE acknowledged_on IS NULL;

CREATE OR REPLACE TRIGGER trg_plg_org_platform_updated_at BEFORE UPDATE ON plg_org_platform
    FOR EACH ROW EXECUTE FUNCTION plg_set_updated_at();

-- A pairing moves forward, or out. Never back, and never out of ABANDONED.
--
-- The five stages before ABANDONED are a genuine progression — a customer does
-- not un-achieve first value — so a backwards move is a mistake rather than a
-- correction, and the schema says so.
--
-- Enforced here rather than only in the service layer because it is an
-- invariant of the data, not a workflow rule — a correction applied straight to
-- the table should fail too. The BFF checks it first anyway, to produce a
-- readable message instead of a trigger's.
CREATE OR REPLACE FUNCTION plg_check_stage_move() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    v_from INT;
    v_to   INT;
BEGIN
    IF NEW.lifecycle_stage = OLD.lifecycle_stage THEN
        RETURN NEW;
    END IF;

    IF OLD.lifecycle_stage = 'ABANDONED' THEN
        RAISE EXCEPTION 'a pairing cannot leave ABANDONED'
            USING ERRCODE = 'check_violation';
    END IF;

    -- Out is always allowed. A customer can give up at any point, so this is
    -- the one move that ignores the ordering.
    IF NEW.lifecycle_stage = 'ABANDONED' THEN
        RETURN NEW;
    END IF;

    SELECT display_order INTO v_from FROM plg_lifecycle_stage WHERE stage = OLD.lifecycle_stage;
    SELECT display_order INTO v_to   FROM plg_lifecycle_stage WHERE stage = NEW.lifecycle_stage;

    IF v_to <= v_from THEN
        RAISE EXCEPTION 'a pairing moves forward only: % cannot follow %',
            NEW.lifecycle_stage, OLD.lifecycle_stage
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE TRIGGER trg_plg_org_platform_stage_move
    BEFORE UPDATE OF lifecycle_stage ON plg_org_platform
    FOR EACH ROW EXECUTE FUNCTION plg_check_stage_move();

-- One row per change of state, per pairing — of any of the three axes.
--
-- A stage move, a health change and a tier change are the same kind of event:
-- an engineer recording a judgement about the pairing, with a reason and a name
-- against it. The product tab shows one timeline, so they belong in one table
-- rather than in three that have to be merged at read time.
--
-- Every "to_" column is nullable. A row records a stage move, a health move, a
-- subscription change, or several at once; the CHECK below refuses a row that
-- records none of them. The genesis row
-- still has from_stage NULL and is written at ingest, so the history is never
-- blank.
CREATE TABLE IF NOT EXISTS plg_lifecycle_history (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_platform_id  UUID NOT NULL REFERENCES plg_org_platform (id) ON DELETE CASCADE,

    from_stage       plg_lifecycle_stage_enum REFERENCES plg_lifecycle_stage (stage),
    to_stage         plg_lifecycle_stage_enum REFERENCES plg_lifecycle_stage (stage),

    from_health      plg_health_enum,
    to_health        plg_health_enum,

    from_subscription plg_subscription_tier_enum,
    to_subscription   plg_subscription_tier_enum,

    -- NOT NULL, for every axis including health. A change recorded without a
    -- reason is the one kind of history entry nobody can act on: the timeline
    -- shows a name, a date and a blank. It is tempting to exempt health, on the
    -- theory that demanding a sentence stops engineers marking risk at all --
    -- but an unexplained risk mark is exactly what the next engineer has to
    -- phone someone about, so the sentence is cheaper than its absence.
    reason           TEXT NOT NULL,
    changed_by       UUID REFERENCES "user" (id) ON DELETE SET NULL,
    changed_on       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- A row that changed nothing has nothing to say. Without this, a bug that
    -- writes an empty history row is invisible until someone reads the timeline
    -- and finds a blank entry with a name and a date on it.
    CONSTRAINT chk_plg_history_records_something
        CHECK (to_stage IS NOT NULL OR to_health IS NOT NULL OR to_subscription IS NOT NULL),

    -- NOT NULL alone would accept a space. The column exists to be read.
    CONSTRAINT chk_plg_history_reason_not_blank
        CHECK (BTRIM(reason) <> '')
);

CREATE INDEX IF NOT EXISTS idx_plg_lifecycle_history_pairing
    ON plg_lifecycle_history (org_platform_id, changed_on DESC);

-- ---------------------------------------------------------------------------
-- Playbook templates
-- ---------------------------------------------------------------------------

-- A playbook belongs to one product and one source stage. Its job is to move a
-- pairing along one of the seven paths.
CREATE TABLE IF NOT EXISTS plg_playbook (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id      UUID NOT NULL REFERENCES plg_product (id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    description     TEXT,

    -- Where it applies. A progressive playbook run here carries the pairing to
    -- the next stage; a risk-intervention one restores its health without
    -- moving it. There is no target column: the type says where it is going.
    lifecycle_stage plg_lifecycle_stage_enum NOT NULL
                        REFERENCES plg_lifecycle_stage (stage),

    -- Which of the two axes this playbook works on. NOT NULL and with no
    -- default: an author must choose, because the choice decides which pairings
    -- are ever offered it.
    playbook_type   plg_playbook_type_enum NOT NULL,

    display_order   INT         NOT NULL DEFAULT 0,
    active          BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_plg_playbook_name UNIQUE (product_id, name),

    -- Nothing progresses out of ABANDONED, so a progressive playbook there
    -- could never run. A risk-intervention one is equally meaningless: the
    -- pairing is gone. Refusing both here is cheaper than explaining later why
    -- an authored playbook never appears.
    CONSTRAINT chk_plg_playbook_not_abandoned
        CHECK (lifecycle_stage <> 'ABANDONED')
);

-- No foreign key ties a playbook to a path through the stages, and none is
-- wanted: every stage carries playbooks, a playbook has no target stage to
-- validate, and its kind already implies where it leads.
--
-- What is left is duller and smaller: a plain FK on the stage, NOT NULL on the
-- type, and the CHECK above.

CREATE INDEX IF NOT EXISTS idx_plg_playbook_product_stage
    ON plg_playbook (product_id, lifecycle_stage, display_order);

CREATE OR REPLACE TRIGGER trg_plg_playbook_updated_at BEFORE UPDATE ON plg_playbook
    FOR EACH ROW EXECUTE FUNCTION plg_set_updated_at();

-- Template tasks. `code` is the stable identity across template revisions, so
-- renaming a task keeps its meaning; the two bookend codes are reserved.
CREATE TABLE IF NOT EXISTS plg_playbook_task (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    playbook_id UUID NOT NULL REFERENCES plg_playbook (id) ON DELETE CASCADE,
    code        TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    sequence_no INT  NOT NULL,
    value_type  plg_task_value_type_enum NOT NULL DEFAULT 'BOOLEAN',
    active      BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- The answers a CHECKLIST or SINGLE_SELECT task offers:
    -- [{"code","label"}, …]. JSONB rather than a child table because they are
    -- authored, reordered and rewritten as one list, and nothing ever queries a
    -- single option on its own.
    options     JSONB,

    CONSTRAINT uq_plg_playbook_task_code UNIQUE (playbook_id, code),
    -- Deferrable so a reorder can happen in one transaction.
    CONSTRAINT uq_plg_playbook_task_seq  UNIQUE (playbook_id, sequence_no)
        DEFERRABLE INITIALLY DEFERRED,

    -- The bookends are structural, so their type is not the author's to choose.
    CONSTRAINT chk_plg_playbook_task_bookend_boolean
        CHECK (code NOT IN ('INITIATE_PLAYBOOK', 'CLOSE_PLAYBOOK')
               OR value_type = 'BOOLEAN'),

    -- A list with nothing to choose could never be completed; with one option
    -- it is a tick box wearing a costume. That holds for both list types.
    -- Everything else must have no options at all, so the column cannot quietly
    -- accumulate meaning on types that ignore it.
    CONSTRAINT chk_plg_playbook_task_options CHECK (
        CASE WHEN value_type IN ('CHECKLIST', 'SINGLE_SELECT')
             THEN options IS NOT NULL
                  AND jsonb_typeof(options) = 'array'
                  AND jsonb_array_length(options) >= 2
             ELSE options IS NULL
        END
    )
);

CREATE OR REPLACE TRIGGER trg_plg_playbook_task_updated_at BEFORE UPDATE ON plg_playbook_task
    FOR EACH ROW EXECUTE FUNCTION plg_set_updated_at();

-- ---------------------------------------------------------------------------
-- Playbook execution
-- ---------------------------------------------------------------------------

-- One playbook running against one pairing.
CREATE TABLE IF NOT EXISTS plg_playbook_run (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_platform_id UUID NOT NULL REFERENCES plg_org_platform (id) ON DELETE CASCADE,
    playbook_id     UUID NOT NULL REFERENCES plg_playbook (id)     ON DELETE RESTRICT,
    added_by        UUID REFERENCES "user" (id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_plg_playbook_run UNIQUE (org_platform_id, playbook_id)
);

CREATE INDEX IF NOT EXISTS idx_plg_playbook_run_playbook ON plg_playbook_run (playbook_id);

CREATE OR REPLACE TRIGGER trg_plg_playbook_run_updated_at BEFORE UPDATE ON plg_playbook_run
    FOR EACH ROW EXECUTE FUNCTION plg_set_updated_at();

-- A playbook must belong to the same product as the pairing running it.
CREATE OR REPLACE FUNCTION plg_check_run_product() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    v_pairing_product  UUID;
    v_playbook_product UUID;
BEGIN
    SELECT product_id INTO v_pairing_product  FROM plg_org_platform WHERE id = NEW.org_platform_id;
    SELECT product_id INTO v_playbook_product FROM plg_playbook     WHERE id = NEW.playbook_id;

    IF v_pairing_product IS DISTINCT FROM v_playbook_product THEN
        RAISE EXCEPTION 'playbook % belongs to a different product than pairing %',
            NEW.playbook_id, NEW.org_platform_id
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE TRIGGER trg_plg_playbook_run_product
    BEFORE INSERT OR UPDATE OF playbook_id, org_platform_id ON plg_playbook_run
    FOR EACH ROW EXECUTE FUNCTION plg_check_run_product();

-- Task instances. These are COPIES of the template task, never references:
-- editing a playbook can therefore never silently rewrite work already recorded.
-- Template edits reach new runs only; runs in flight keep the list they started
-- with, and there are no ad-hoc tasks, so a run differing from its template has
-- exactly one explanation.
CREATE TABLE IF NOT EXISTS plg_playbook_run_task (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    playbook_run_id  UUID NOT NULL REFERENCES plg_playbook_run (id)  ON DELETE CASCADE,
    playbook_task_id UUID NOT NULL REFERENCES plg_playbook_task (id) ON DELETE RESTRICT,

    code        TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    sequence_no INT  NOT NULL,
    value_type  plg_task_value_type_enum NOT NULL,

    -- Exactly one of these is ever populated, and it is the one value_type names.
    --
    -- SINGLE_SELECT stores the chosen option's CODE in value_text rather than
    -- taking a column of its own. It is a single scalar answer, which is what
    -- value_text is; what distinguishes it from STRING is not the storage but
    -- the options beside it, which constrain what may be written there.
    value_bool   BOOLEAN,
    value_number NUMERIC,
    value_text   TEXT,

    completed_on   TIMESTAMPTZ,
    completed_by   UUID REFERENCES "user" (id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Copied from the template alongside name and type, so a template edited
    -- tomorrow cannot change the reasons a run already offered.
    options        JSONB,

    -- The codes actually ticked. TEXT[] rather than JSONB so it takes a GIN
    -- index and unnests cleanly.
    value_checked  TEXT[],

    -- Completion is derived, not stored, so a status and a value cannot
    -- disagree. A boolean task is complete when TRUE — a recorded FALSE reads
    -- as unanswered. A reason picker is complete once *any* reason is ticked:
    -- you record why, and one reason is a why. Requiring all of them would mean
    -- a disqualification had to be true for every listed reason at once.
    is_completed   BOOLEAN GENERATED ALWAYS AS (
        CASE value_type
            WHEN 'BOOLEAN'   THEN value_bool IS TRUE
            WHEN 'NUMBER'    THEN value_number IS NOT NULL
            WHEN 'STRING'        THEN value_text IS NOT NULL AND btrim(value_text) <> ''
            WHEN 'SINGLE_SELECT' THEN value_text IS NOT NULL AND btrim(value_text) <> ''
            WHEN 'CHECKLIST'     THEN value_checked IS NOT NULL AND cardinality(value_checked) > 0
        END
    ) STORED,

    CONSTRAINT uq_plg_run_task_code UNIQUE (playbook_run_id, code),

    -- Without this a STRING task could quietly carry a number nobody reads.
    -- Exactly one value column is populated, and it is the one value_type names.
    CONSTRAINT chk_plg_run_task_value_type CHECK (
        CASE value_type
            WHEN 'BOOLEAN'   THEN value_number IS NULL AND value_text IS NULL AND value_checked IS NULL
            WHEN 'NUMBER'    THEN value_bool   IS NULL AND value_text IS NULL AND value_checked IS NULL
            WHEN 'STRING'        THEN value_bool IS NULL AND value_number IS NULL AND value_checked IS NULL
            WHEN 'SINGLE_SELECT' THEN value_bool IS NULL AND value_number IS NULL AND value_checked IS NULL
            WHEN 'CHECKLIST'     THEN value_bool IS NULL AND value_number IS NULL AND value_text   IS NULL
        END
    ),

    CONSTRAINT chk_plg_run_task_options CHECK (
        CASE WHEN value_type IN ('CHECKLIST', 'SINGLE_SELECT')
             THEN options IS NOT NULL
                  AND jsonb_typeof(options) = 'array'
                  AND jsonb_array_length(options) >= 2
             ELSE options IS NULL
        END
    )
);

CREATE INDEX IF NOT EXISTS idx_plg_run_task_run  ON plg_playbook_run_task (playbook_run_id, sequence_no);
CREATE INDEX IF NOT EXISTS idx_plg_run_task_open ON plg_playbook_run_task (playbook_run_id)
    WHERE NOT is_completed;
-- "How many were disqualified for an unreachable email?" is an index scan, not
-- a sequential one.
CREATE INDEX IF NOT EXISTS idx_plg_run_task_checked ON plg_playbook_run_task USING GIN (value_checked);

CREATE OR REPLACE TRIGGER trg_plg_run_task_updated_at BEFORE UPDATE ON plg_playbook_run_task
    FOR EACH ROW EXECUTE FUNCTION plg_set_updated_at();

-- ---------------------------------------------------------------------------
-- Comment trail
-- ---------------------------------------------------------------------------

-- The comment trail. Editable by its author, and every edit is recorded rather
-- than merely permitted: the body a note had goes to PLG_NOTE_REVISION before
-- being overwritten, so the trail stays truthful about what was understood in
-- July even after August corrected it. No delete. This is also where
-- customer-specific work lands, now that ad-hoc tasks are ruled out.
CREATE TABLE IF NOT EXISTS plg_note (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_platform_id UUID NOT NULL REFERENCES plg_org_platform (id) ON DELETE CASCADE,
    body            TEXT NOT NULL,
    author          UUID REFERENCES "user" (id) ON DELETE SET NULL,
    created_on      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- NULL means never edited. Nothing stores an "edited" flag, so nothing can
    -- disagree with the revisions beside it.
    updated_on      TIMESTAMPTZ,
    updated_by      UUID REFERENCES "user" (id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_plg_note_pairing ON plg_note (org_platform_id, created_on DESC);

-- Authorship is checked on edit, so it is read on every note PATCH.
CREATE INDEX IF NOT EXISTS idx_plg_note_author  ON plg_note (author);

-- ---------------------------------------------------------------------------
-- Added after the core: the overflow table, the ingest log, note revisions
-- ---------------------------------------------------------------------------

-- Overflow storage for source fields beyond the fourteen the portal models.
--
-- The fourteen core fields stay in their typed columns on plg_organization and
-- plg_org_platform: every list, filter and chart reads them, and name/value
-- pairs would make ordinary queries slow and awkward. This table is for the
-- rest — fields the source sends that the portal has no column for, kept so they
-- are available when someone later wants to analyse one.
--
-- Only fields named in the attribute map are stored (backend/source-map.json).
-- Everything else on the payload is ignored.
CREATE TABLE IF NOT EXISTS plg_organization_attribute (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Always set. An attribute belongs to a customer.
    organization_id UUID NOT NULL REFERENCES plg_organization (id) ON DELETE CASCADE,

    -- Set when the attribute is about one platform rather than the customer as a
    -- whole. NULL is meaningful here — it is what "organisation level" looks
    -- like — which is why the unique constraint below treats NULLs as equal.
    org_platform_id UUID REFERENCES plg_org_platform (id) ON DELETE CASCADE,

    -- The portal-side name from the attribute map, not the source's own, so a
    -- rename upstream does not change what queries here look for.
    attribute_name  TEXT NOT NULL,

    -- Stored exactly as it arrived. Text because the portal makes no claim about
    -- what these mean; anything that needs a number casts at read time.
    attribute_value TEXT NOT NULL,

    -- The source field this came from. Kept for troubleshooting: it is the
    -- only way to tell which upstream field produced a value after a map edit.
    source_field    TEXT NOT NULL,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- NULLS NOT DISTINCT is doing real work. Without it Postgres treats every
    -- NULL org_platform_id as unique, so an organisation-level attribute would
    -- accumulate a new row on every delivery instead of updating in place.
    CONSTRAINT uq_plg_organization_attribute
        UNIQUE NULLS NOT DISTINCT (organization_id, org_platform_id, attribute_name)
);

CREATE INDEX IF NOT EXISTS idx_plg_org_attribute_org  ON plg_organization_attribute (organization_id);
CREATE INDEX IF NOT EXISTS idx_plg_org_attribute_pair ON plg_organization_attribute (org_platform_id)
    WHERE org_platform_id IS NOT NULL;

-- Analysis reads this one far more often than the primary key: "every
-- organisation whose lead source was X".
CREATE INDEX IF NOT EXISTS idx_plg_org_attribute_lookup
    ON plg_organization_attribute (attribute_name, attribute_value);

CREATE OR REPLACE TRIGGER trg_plg_organization_attribute_updated_at
    BEFORE UPDATE ON plg_organization_attribute
    FOR EACH ROW EXECUTE FUNCTION plg_set_updated_at();

-- Events the portal consumed from the queue but could not turn into a
-- registration.
--
-- This table exists because of one property of the queue: consuming deletes.
-- An event handed to this portal is gone from the queue immediately, with no
-- ack and no redelivery. So an event that fails to process — a malformed
-- payload, an unknown platform, an event type this portal does not handle — has
-- nowhere left to exist. Without this table it would be lost, and the customer
-- behind it would simply never appear in the portal.
--
-- Rows here are meant to be looked at. A non-empty table means registrations
-- were delivered and not recorded.
CREATE TABLE IF NOT EXISTS plg_ingest_failure (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- The queue's own event id, so a row here can be matched against the
    -- queue service's logs.
    event_id     TEXT,
    event_type   TEXT,
    received_at  TIMESTAMPTZ,

    -- Exactly what arrived. Enough to replay the event by hand once whatever
    -- caused the failure is fixed.
    payload      JSONB NOT NULL,

    -- Why it could not be processed, in the words the service layer used.
    failure      TEXT NOT NULL,

    -- Set when someone has dealt with it, so the table can be worked through
    -- rather than just growing.
    resolved_on  TIMESTAMPTZ,
    resolved_by  UUID REFERENCES "user" (id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The query this table exists to answer: what is still outstanding?
CREATE INDEX IF NOT EXISTS idx_plg_ingest_failure_open ON plg_ingest_failure (created_at DESC)
    WHERE resolved_on IS NULL;

-- One row per edit, holding the body as it was *before* that edit — not after.
-- Reading them in edited_on order therefore replays the note's history, and the
-- current body is the one on PLG_NOTE rather than the newest revision.
CREATE TABLE IF NOT EXISTS plg_note_revision (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    note_id   UUID        NOT NULL REFERENCES plg_note (id) ON DELETE CASCADE,
    body      TEXT        NOT NULL,
    edited_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    edited_by UUID        REFERENCES "user" (id) ON DELETE SET NULL
);

COMMENT ON TABLE plg_note_revision IS
    'Superseded note bodies. Each row is what the note said before the edit that created the row.';

-- The only query this table serves: one note's history, oldest first.
CREATE INDEX IF NOT EXISTS idx_plg_note_revision_note ON plg_note_revision (note_id, edited_on);

COMMENT ON COLUMN plg_note.updated_on IS
    'When the body was last changed. NULL means never edited.';

-- ---------------------------------------------------------------------------
-- Read models
-- ---------------------------------------------------------------------------
--
-- An owner is a UUID, which is meaningless on a screen. So every view that
-- exposes an owner exposes three columns rather than one: the id (for joins and
-- filters), the email and the display name (for rendering). Callers never join
-- `"user"` themselves; the view has already done it.
--
-- That is why plg_work_queue_v carries owner columns of its own. With the join
-- in the view there is one definition of "the owner's name" instead of one per
-- query that happens to need it.

-- A CS engineer as the portal renders one: the surrogate that identifies, plus
-- the two fields that display.
--
-- An owner, an author, an acknowledger and a task's completer are each a UUID,
-- and every query that shows a person needs the id, the address and a name
-- assembled from first_name and last_name. Assembling it once here beats
-- repeating the COALESCE in every view that renders someone.
--
-- Without this view that assembly would be repeated nine times across this file
-- and every query that joins a person — nine chances for one of them to drift
-- into rendering a name differently. Joining this instead means "the engineer's
-- display name" has exactly one definition, and it is here.
-- THIS VIEW DOES NOT FILTER BY user_type, AND MUST NOT.
--
-- PLG only ever *selects* internal users — the caller resolving their own
-- identity, the owner pickers, the engineer list. That restriction lives in
-- plg_users_repo.go, on the search path.
--
-- This is the other question: rendering the name of whoever did something,
-- historically. A note written last year by an engineer who has since been
-- reclassified, or deactivated, must still render their name — the alternative
-- is a blank label on a row that plainly has an author. Filtering here would
-- silently erase attribution, which is the opposite of what the audit columns
-- are for.
CREATE OR REPLACE VIEW plg_user_v AS
SELECT u.id,
       u.email,
       -- `name` first, because upstream populates it and it is the name as the
       -- person wrote it. Falling back to first+last covers the rows that have
       -- the parts but not the whole. COALESCE on each part because every one
       -- of these columns is nullable in upstream's shape: concatenating a NULL
       -- would yield NULL, and a NULL label is a blank on screen rather than an
       -- error anyone notices.
       COALESCE(
           NULLIF(TRIM(BOTH ' ' FROM COALESCE(u.name, '')), ''),
           NULLIF(TRIM(BOTH ' ' FROM COALESCE(u.first_name, '') || ' ' ||
                                     COALESCE(u.last_name, '')), '')
       ) AS display_name,
       u.is_active,
       u.user_type
FROM   "user" u;

CREATE OR REPLACE VIEW plg_playbook_run_v AS
SELECT r.id                                    AS playbook_run_id,
       r.org_platform_id,
       r.playbook_id,
       r.added_by,
       r.created_at,
       pb.name                                 AS playbook_name,
       pb.description                          AS playbook_description,
       pb.lifecycle_stage                      AS playbook_stage,
       pb.playbook_type,
       op.organization_id,
       o.organization_name,
       o.plg_cs_owner,
       op.product_id,
       pr.code                                 AS product_code,
       pr.name                                 AS product_name,
       op.lifecycle_stage                      AS current_stage,
       COUNT(t.id)                             AS task_total,
       COUNT(t.id) FILTER (WHERE t.is_completed) AS task_completed,
       CASE
           WHEN bool_or(t.code = 'CLOSE_PLAYBOOK'    AND t.is_completed) THEN 'CLOSED'
           WHEN bool_or(t.code = 'INITIATE_PLAYBOOK' AND t.is_completed) THEN 'ACTIVE'
           ELSE 'NOT_STARTED'
       END                                     AS run_status,
       (ARRAY_AGG(t.code ORDER BY t.sequence_no)
          FILTER (WHERE NOT t.is_completed))[1] AS next_task_code,
       (ARRAY_AGG(t.name ORDER BY t.sequence_no)
          FILTER (WHERE NOT t.is_completed))[1] AS next_task_name
FROM   plg_playbook_run r
JOIN   plg_playbook pb     ON pb.id = r.playbook_id
JOIN   plg_org_platform op ON op.id = r.org_platform_id
JOIN   plg_organization o  ON o.id = op.organization_id
JOIN   plg_product pr      ON pr.id = op.product_id
LEFT   JOIN plg_playbook_run_task t ON t.playbook_run_id = r.id
GROUP  BY r.id, r.org_platform_id, r.playbook_id, r.added_by, r.created_at,
          pb.name, pb.description, pb.lifecycle_stage, pb.playbook_type,
          op.organization_id, o.organization_name, o.plg_cs_owner,
          op.product_id, pr.code, pr.name, op.lifecycle_stage;

CREATE OR REPLACE VIEW plg_organization_v AS
SELECT o.id                    AS organization_id,
       o.organization_name,
       o.created_on,
       o.registered_user,
       p.email                 AS registered_email,
       NULLIF(TRIM(BOTH ' ' FROM COALESCE(p.first_name, '') || ' ' ||
                                 COALESCE(p.last_name, '')), '') AS registered_name,

       -- The owner, as three columns. `plg_cs_owner` is the surrogate the
       -- FK stores; the other two exist so no caller has to join "user" to
       -- render a name.
       o.plg_cs_owner,
       u.email                 AS plg_cs_owner_email,
       u.display_name          AS plg_cs_owner_name,
       u.is_active                AS plg_cs_owner_active,

       o.moesif_company_id,

       COALESCE(o.country_name, (
           SELECT o2.country_name FROM plg_organization o2
           WHERE o2.registered_user = o.registered_user AND o2.country_name IS NOT NULL
           ORDER BY o2.created_on LIMIT 1))                      AS country_name,

       COALESCE(o.company_name_from_domain, (
           SELECT o2.company_name_from_domain FROM plg_organization o2
           WHERE o2.registered_user = o.registered_user AND o2.company_name_from_domain IS NOT NULL
           ORDER BY o2.created_on LIMIT 1))                      AS company_name_from_domain,

       -- True when this organisation's own row was sparse and the values above
       -- came from a sibling registered by the same person.
       --
       -- Country is the test because it is the only resolvable field the source
       -- still sends.
       (o.country_name IS NULL)                                  AS fields_inherited
FROM   plg_organization o
JOIN   plg_person p  ON p.id = o.registered_user
-- Joined on the surrogate, never on the address: email is carried for display
-- and is not a key.
LEFT   JOIN plg_user_v u ON u.id = o.plg_cs_owner;

CREATE OR REPLACE VIEW plg_org_platform_v AS
SELECT op.id                     AS org_platform_id,
       op.organization_id,
       v.organization_name,
       v.registered_email,
       v.registered_name,
       v.plg_cs_owner,
       v.plg_cs_owner_email,
       v.plg_cs_owner_name,
       op.product_id,
       pr.code                   AS product_code,
       pr.name                   AS product_name,
       pr.display_order          AS product_display_order,
       op.registered_on,
       op.lifecycle_stage,
       ls.name                   AS lifecycle_stage_name,
       ls.display_order          AS lifecycle_stage_order,
       op.stage_entered_on,
       op.acknowledged_on,
       op.acknowledged_by,
       ack.email                 AS acknowledged_by_email,
       ack.display_name          AS acknowledged_by_name,

       -- The second axis, with the same three-column treatment the owner gets:
       -- the value, and the person who last set it, resolved for display.
       op.health_state,
       op.health_entered_on,
       op.health_updated_by,
       hu.email                  AS health_updated_by_email,
       hu.display_name           AS health_updated_by_name,

       -- The third axis, with the same treatment the other two get.
       op.subscription_tier,
       op.subscription_entered_on,
       op.subscription_updated_by,
       su.email                  AS subscription_updated_by_email,
       su.display_name           AS subscription_updated_by_name,

       op.trial_end_date,
       op.trial_extended_date,
       plg_current_period_end_date(op.subscription_tier, op.trial_end_date,
                                   op.trial_extended_date) AS current_period_end_date,
       (op.acknowledged_on IS NULL)                                  AS is_new,

       -- Which kinds of playbook this pairing is currently offered. Derived
       -- from health rather than stored, so it cannot disagree with it. An
       -- array because a healthy pairing is offered progressive AND sustaining
       -- work — the answer is not a single value.
       plg_applicable_playbook_types(op.health_state)::TEXT[]        AS applicable_playbook_types,

       -- Every stage carries playbooks, so the useful question is whether one
       -- actually EXISTS for this product, stage and kind. A stage with none
       -- authored is an author's problem, and this is what surfaces it.
       EXISTS (SELECT 1 FROM plg_playbook pb
               WHERE pb.product_id      = op.product_id
                 AND pb.lifecycle_stage = op.lifecycle_stage
                 AND pb.active
                 AND pb.playbook_type   = ANY(
                         plg_applicable_playbook_types(op.health_state)))
                                                                     AS stage_carries_playbooks,
       COALESCE(rs.run_total, 0)                                     AS run_total,
       COALESCE(rs.run_active, 0)                                    AS run_active,
       COALESCE(rs.run_closed, 0)                                    AS run_closed,
       COALESCE(rs.task_total, 0)                                    AS task_total,
       COALESCE(rs.task_completed, 0)                                AS task_completed
FROM   plg_org_platform op
JOIN   plg_organization_v v ON v.organization_id = op.organization_id
JOIN   plg_product pr       ON pr.id = op.product_id
JOIN   plg_lifecycle_stage ls ON ls.stage = op.lifecycle_stage
-- Who acknowledged the registration, resolved here for the same reason the
-- owner is: the column holds a surrogate, the panel shows a person.
LEFT   JOIN plg_user_v ack ON ack.id = op.acknowledged_by
LEFT   JOIN plg_user_v hu  ON hu.id  = op.health_updated_by
LEFT   JOIN plg_user_v su  ON su.id  = op.subscription_updated_by
LEFT   JOIN LATERAL (
           SELECT COUNT(*)                                              AS run_total,
                  COUNT(*) FILTER (WHERE rv.run_status = 'ACTIVE')      AS run_active,
                  COUNT(*) FILTER (WHERE rv.run_status = 'CLOSED')      AS run_closed,
                  COALESCE(SUM(rv.task_total), 0)                       AS task_total,
                  COALESCE(SUM(rv.task_completed), 0)                   AS task_completed
           FROM   plg_playbook_run_v rv
           WHERE  rv.org_platform_id = op.id
       ) rs ON TRUE;

CREATE OR REPLACE VIEW plg_work_queue_v AS
SELECT op.id                       AS org_platform_id,
       op.organization_id,
       o.organization_name,

       -- Three owner columns, for the reason given at the top of this section.
       o.plg_cs_owner,
       ow.email                    AS plg_cs_owner_email,
       ow.display_name             AS plg_cs_owner_name,

       op.product_id,
       pr.code                     AS product_code,
       pr.name                     AS product_name,
       pr.display_order            AS product_display_order,
       op.lifecycle_stage,
       ls.name                     AS lifecycle_stage_name,
       ls.display_order            AS lifecycle_stage_order,
       op.acknowledged_on,
       op.acknowledged_by,
       ack.email                   AS acknowledged_by_email,
       ack.display_name            AS acknowledged_by_name,

       -- The second axis, surfaced on every queue row. An at-risk pairing is
       -- the most valuable thing in the queue and the list has to be able to
       -- say so without a second query.
       op.health_state,
       op.health_entered_on,

       COALESCE(runs.run_total, 0)   AS run_total,
       COALESCE(runs.run_active, 0)  AS run_active,
       COALESCE(runs.run_closed, 0)  AS run_closed,
       COALESCE(runs.task_total, 0)     AS task_total,
       COALESCE(runs.task_completed, 0) AS task_completed,
       avail.available_total,

       -- Why this pairing is in the queue, most advanced state first. A pairing
       -- with one run under way and another untouched reads as IN_PROGRESS:
       -- something is moving, which is the more useful thing to know.
       --
       -- There is no "finished" reason, deliberately. Everything closed with
       -- nothing left to attach is the EXIT condition — see the WHERE clause at
       -- the bottom — so a pairing with nothing left to do leaves the queue
       -- rather than sitting in it looking like work.
       CASE
           WHEN COALESCE(runs.run_active, 0) > 0                           THEN 'IN_PROGRESS'
           WHEN COALESCE(runs.run_total, 0) > COALESCE(runs.run_closed, 0) THEN 'NOT_STARTED'
           ELSE 'NO_PLAYBOOK'
       END AS reason,

       runs.next_task_code,
       runs.next_task_name,
       runs.playbook_id,
       runs.playbook_name,
       op.registered_on
FROM   plg_org_platform op
JOIN   plg_organization o     ON o.id  = op.organization_id
JOIN   plg_product pr         ON pr.id = op.product_id
JOIN   plg_lifecycle_stage ls ON ls.stage = op.lifecycle_stage
LEFT   JOIN plg_user_v ow     ON ow.id = o.plg_cs_owner
LEFT   JOIN plg_user_v ack    ON ack.id = op.acknowledged_by

-- What is running on this pairing, and what the engineer would pick up next.
LEFT   JOIN LATERAL (
           SELECT COUNT(*)                                                  AS run_total,
                  COUNT(*) FILTER (WHERE rv.run_status = 'ACTIVE')          AS run_active,
                  COUNT(*) FILTER (WHERE rv.run_status = 'CLOSED')          AS run_closed,
                  COALESCE(SUM(rv.task_total), 0)                           AS task_total,
                  COALESCE(SUM(rv.task_completed), 0)                       AS task_completed,
                  (ARRAY_AGG(rv.next_task_code ORDER BY rv.created_at)
                     FILTER (WHERE rv.run_status = 'ACTIVE'))[1]            AS next_task_code,
                  (ARRAY_AGG(rv.next_task_name ORDER BY rv.created_at)
                     FILTER (WHERE rv.run_status = 'ACTIVE'))[1]            AS next_task_name,
                  (ARRAY_AGG(rv.playbook_id ORDER BY rv.created_at)
                     FILTER (WHERE rv.run_status = 'ACTIVE'))[1]            AS playbook_id,
                  (ARRAY_AGG(rv.playbook_name ORDER BY rv.created_at)
                     FILTER (WHERE rv.run_status = 'ACTIVE'))[1]            AS playbook_name
           FROM   plg_playbook_run_v rv
           WHERE  rv.org_platform_id = op.id
       ) runs ON TRUE

-- How many playbooks this pairing could still add, at its current stage AND of
-- the kind its health calls for. Zero with nothing attached is the case an
-- author has to fix, not an engineer.
--
-- The type filter is what makes health drive the queue. A COMMERCIAL pairing
-- that has finished its progressive and sustaining playbooks counts zero and
-- leaves; mark it AT_RISK and the recovery playbooks at COMMERCIAL start
-- counting, so it returns on its own. Nothing special-cases that.
LEFT   JOIN LATERAL (
           SELECT COUNT(*) AS available_total
           FROM   plg_playbook pb
           WHERE  pb.product_id      = op.product_id
             AND  pb.lifecycle_stage = op.lifecycle_stage
             AND  pb.active
             AND  pb.playbook_type   = ANY(
                      plg_applicable_playbook_types(op.health_state))
             AND  NOT EXISTS (SELECT 1 FROM plg_playbook_run r
                              WHERE r.org_platform_id = op.id AND r.playbook_id = pb.id)
       ) avail ON TRUE

-- Who is on the list.
--
-- The question is simply: is there anything left to DO?
--
--   acknowledged          nobody works a registration they have not claimed
--   not ABANDONED         terminal; there is no work on a customer who left
--   something to do       a run going, a run untouched, or a playbook that
--                         could still be attached for this health
--
-- The third clause is the exit condition. A pairing with everything closed and
-- nothing attachable is finished — it leaves rather than sitting on the list
-- looking like work.
--
-- "Nothing attached yet" is NOT an exit: no runs and an available playbook
-- counts as work, and shows as NO_PLAYBOOK. That is the state every freshly
-- acknowledged registration is in, and it is exactly what an engineer opens the
-- queue to find.
WHERE  op.acknowledged_on IS NOT NULL
  AND  op.lifecycle_stage <> 'ABANDONED'
  AND  ( COALESCE(runs.run_active, 0) > 0
      OR COALESCE(runs.run_total, 0) > COALESCE(runs.run_closed, 0)
      OR COALESCE(avail.available_total, 0) > 0 );

-- The relational surface a child table would have provided, derived instead of
-- stored — so it cannot drift from the task it describes, and no second write
-- path has to keep it in step.
--
--   SELECT reason_label, COUNT(*) FROM plg_run_task_reason_v
--   WHERE  task_code = 'DISQUALIFY_EMAIL_DOMAIN' GROUP BY 1;
--
-- BOTH LIST TYPES REPORT THROUGH HERE. A CHECKLIST contributes one row per
-- ticked code; a SINGLE_SELECT contributes exactly one. They are unioned rather
-- than left as two views because the question asked of them is identical —
-- "how many chose X" — and a caller should not have to know which kind of
-- control an author happened to pick. `single_choice` is there for the caller
-- that does care.
CREATE OR REPLACE VIEW plg_run_task_reason_v AS
SELECT t.id                AS playbook_run_task_id,
       t.playbook_run_id,
       t.code              AS task_code,
       t.name              AS task_name,
       FALSE               AS single_choice,
       r.code              AS reason_code,
       (SELECT o ->> 'label' FROM jsonb_array_elements(t.options) o
        WHERE  o ->> 'code' = r.code)                       AS reason_label,
       t.completed_on,
       t.completed_by
FROM   plg_playbook_run_task t,
       unnest(t.value_checked) AS r(code)

UNION ALL

SELECT t.id,
       t.playbook_run_id,
       t.code,
       t.name,
       TRUE,
       t.value_text,
       (SELECT o ->> 'label' FROM jsonb_array_elements(t.options) o
        WHERE  o ->> 'code' = t.value_text),
       t.completed_on,
       t.completed_by
FROM   plg_playbook_run_task t
WHERE  t.value_type = 'SINGLE_SELECT'
  AND  t.value_text IS NOT NULL;
