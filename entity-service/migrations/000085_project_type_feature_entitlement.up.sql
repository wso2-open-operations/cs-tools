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

-- Mirrors ServiceNow's ProjectTypeFeatureManager.FEATURE_MATRIX (Script
-- Include backing the x_wso2_customer_0 scoped app's
-- GET /projects/{id}/metadata resource) so GetProjectMetadata's Postgres
-- path can populate ProjectFeatures.Has*Access/AcceptedSeverityValues/
-- *ProductCategories instead of leaving every field at zero value.
--
-- Added directly onto project_type, not a separate table: every row gets the
-- same set of columns (no polymorphism the way e.g. "case" extending
-- work_item has across differently-shaped sub-types), and project_type
-- already exists specifically to hold per-type data -- this is more of that,
-- not a new concept.
--
-- The has_* columns default FALSE (NOT NULL), matching
-- ProjectTypeFeatureManager._getDefaultPermissions' all-false fallback --
-- so a project_type this migration's UPDATE below doesn't touch (Cloud
-- Support - Platformer, Internal, Platformer Subscription, Regular -- the
-- same four FEATURE_MATRIX itself has no entry for) keeps exactly today's
-- all-false behavior with no extra nil-handling needed on the Go side.
--
-- accepted_severity_values/*_product_categories reuse this schema's own
-- existing enum types (case_severity_enum from migration 000018,
-- deployed_product_category_enum from migration 000014) rather than raw
-- integer/text arrays -- this data is categorical the same way case.severity
-- and deployed_product.product_category already are, so it gets the same
-- treatment. case_severity_enum's S0..S4 map onto ServiceNow's own numeric
-- severity ids exactly as case_repo.go's severityToSNLabel does (S0=14
-- Catastrophic, S1=10 Critical, S2=11 High, S3=12 Medium, S4=13 Low) --
-- resolved back to those ids on the Go side, same as before.
ALTER TABLE project_type
    ADD COLUMN has_service_request_write_access BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN has_service_request_read_access BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN has_change_request_read_access BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN has_sra_write_access BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN has_sra_read_access BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN has_engagements_read_access BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN has_updates_read_access BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN has_deployment_write_access BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN has_deployment_read_access BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN has_time_logs_read_access BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN has_component_analysis_read_access BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN has_usage_metrics_read_access BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN accepted_severity_values case_severity_enum[],
    ADD COLUMN default_case_product_categories deployed_product_category_enum[],
    ADD COLUMN sr_product_categories deployed_product_category_enum[];

-- FEATURE_MATRIX's 7 entries, transcribed field-for-field from the real
-- ServiceNow Script Include (verified 2026-09-25 against staging).
--
-- Matched by project_type.name (UNIQUE), not id: ServiceNow sys_ids are
-- generated per-record at creation time, so a project_type row's id in
-- production is not guaranteed to be the same value it is in staging even
-- for "the same" conceptual type -- FEATURE_MATRIX's own sys_id keys were
-- cross-checked against staging's project_type.id values specifically, and
-- that check does not transfer to another environment. Matching by name
-- instead means these UPDATEs apply correctly in any environment where
-- these names exist, rather than silently matching zero rows if sys_ids
-- differ there.
--
-- Each UPDATE is wrapped in its own DO block and checks FOUND immediately
-- after: a bare top-level UPDATE followed by a separate DO block checking
-- FOUND would NOT work -- FOUND is scoped to a single PL/pgSQL execution,
-- so a fresh DO block starts with FOUND unset regardless of what a prior,
-- separate top-level statement did. Putting the UPDATE and the FOUND check
-- inside the same block is what makes this a real guard: a name that
-- doesn't match (typo, prior rename, a future environment with different
-- names) fails the migration loudly instead of leaving every feature flag
-- silently FALSE with no indication anything went wrong.
DO $$
BEGIN
    UPDATE project_type SET
        has_service_request_write_access = TRUE, has_service_request_read_access = TRUE, has_change_request_read_access = TRUE,
        has_sra_write_access = TRUE, has_sra_read_access = TRUE, has_engagements_read_access = TRUE, has_updates_read_access = TRUE,
        has_deployment_write_access = TRUE, has_deployment_read_access = TRUE, has_time_logs_read_access = TRUE,
        has_component_analysis_read_access = TRUE, has_usage_metrics_read_access = TRUE,
        -- Verbatim order from FEATURE_MATRIX's own SEVERITY_ALL constant
        -- (SEVERITY.CRITICAL_P1, HIGH_P2, MEDIUM_P3, LOW_P4, CATASTROPHIC_P0)
        -- -- S1,S2,S3,S4,S0, not S0..S4. Not a mistake; do not "fix" the order.
        accepted_severity_values = ARRAY['S1', 'S2', 'S3', 'S4', 'S0']::case_severity_enum[],
        sr_product_categories = ARRAY['MS', 'PC']::deployed_product_category_enum[]
    WHERE name = 'Managed Cloud Subscription';
    IF NOT FOUND THEN
        RAISE EXCEPTION 'migration 000085: expected project_type row ''Managed Cloud Subscription'' not found -- backfill skipped';
    END IF;
END $$;

DO $$
BEGIN
    UPDATE project_type SET
        has_updates_read_access = TRUE, has_deployment_write_access = TRUE, has_deployment_read_access = TRUE,
        has_time_logs_read_access = TRUE, has_usage_metrics_read_access = TRUE,
        accepted_severity_values = ARRAY['S1', 'S2', 'S3', 'S4']::case_severity_enum[]
    WHERE name = 'Evaluation Subscription';
    IF NOT FOUND THEN
        RAISE EXCEPTION 'migration 000085: expected project_type row ''Evaluation Subscription'' not found -- backfill skipped';
    END IF;
END $$;

DO $$
BEGIN
    UPDATE project_type SET
        has_sra_write_access = TRUE, has_sra_read_access = TRUE, has_engagements_read_access = TRUE, has_updates_read_access = TRUE,
        has_deployment_write_access = TRUE, has_deployment_read_access = TRUE, has_time_logs_read_access = TRUE,
        has_component_analysis_read_access = TRUE, has_usage_metrics_read_access = TRUE,
        accepted_severity_values = ARRAY['S1', 'S2', 'S3', 'S4']::case_severity_enum[]
    WHERE name = 'Subscription';
    IF NOT FOUND THEN
        RAISE EXCEPTION 'migration 000085: expected project_type row ''Subscription'' not found -- backfill skipped';
    END IF;
END $$;

DO $$
BEGIN
    UPDATE project_type SET
        has_updates_read_access = TRUE, has_deployment_write_access = TRUE, has_deployment_read_access = TRUE,
        has_time_logs_read_access = TRUE, has_usage_metrics_read_access = TRUE,
        accepted_severity_values = ARRAY['S4']::case_severity_enum[]
    WHERE name = 'Development Support';
    IF NOT FOUND THEN
        RAISE EXCEPTION 'migration 000085: expected project_type row ''Development Support'' not found -- backfill skipped';
    END IF;
END $$;

-- Cloud Evaluation Support: every has_* flag is FALSE in FEATURE_MATRIX's own
-- entry for this type -- no feature tabs are accessible -- yet
-- acceptedSeverityValues/defaultCaseProductCategories/srProductCategories are
-- still populated there. Confirmed accurate against the source, not a missed
-- boolean: those arrays are simply unused while every flag that would read
-- them is off, presumably a placeholder for a future entitlement upgrade
-- rather than active configuration today.
DO $$
BEGIN
    UPDATE project_type SET
        accepted_severity_values = ARRAY['S1', 'S2', 'S3', 'S4']::case_severity_enum[],
        default_case_product_categories = ARRAY['CL']::deployed_product_category_enum[],
        sr_product_categories = ARRAY['PDP']::deployed_product_category_enum[]
    WHERE name = 'Cloud Evaluation Support';
    IF NOT FOUND THEN
        RAISE EXCEPTION 'migration 000085: expected project_type row ''Cloud Evaluation Support'' not found -- backfill skipped';
    END IF;
END $$;

-- Cloud Support: hasServiceRequestWriteAccess/ReadAccess are marked
-- "PDP-dependent" in the source script's own comment; transcribed as the
-- static FALSE the matrix hardcodes, since no further runtime override logic
-- was visible in what was reviewed. If a Cloud Support project with an
-- active PDP subscription is later found to need Service Request access,
-- that is this row's known gap, not a transcription error.
--
-- Confirmed accurate against the source separately from that known gap:
-- hasEngagementsReadAccess is the ONLY true flag for this type in
-- FEATURE_MATRIX -- hasUpdatesReadAccess/hasDeploymentReadAccess/
-- hasTimeLogsReadAccess/hasUsageMetricsReadAccess are all FALSE there too,
-- even though Professional Services (also service-oriented) has all of
-- those enabled. Not a missed flag; Cloud Support and Professional Services
-- are simply configured differently in ServiceNow.
DO $$
BEGIN
    UPDATE project_type SET
        has_engagements_read_access = TRUE,
        accepted_severity_values = ARRAY['S1', 'S2', 'S3', 'S4']::case_severity_enum[],
        default_case_product_categories = ARRAY['CL']::deployed_product_category_enum[],
        sr_product_categories = ARRAY['PDP']::deployed_product_category_enum[]
    WHERE name = 'Cloud Support';
    IF NOT FOUND THEN
        RAISE EXCEPTION 'migration 000085: expected project_type row ''Cloud Support'' not found -- backfill skipped';
    END IF;
END $$;

DO $$
BEGIN
    UPDATE project_type SET
        has_engagements_read_access = TRUE, has_updates_read_access = TRUE,
        has_deployment_write_access = TRUE, has_deployment_read_access = TRUE, has_time_logs_read_access = TRUE,
        has_usage_metrics_read_access = TRUE,
        accepted_severity_values = ARRAY['S4']::case_severity_enum[]
    WHERE name = 'Professional Services';
    IF NOT FOUND THEN
        RAISE EXCEPTION 'migration 000085: expected project_type row ''Professional Services'' not found -- backfill skipped';
    END IF;
END $$;
