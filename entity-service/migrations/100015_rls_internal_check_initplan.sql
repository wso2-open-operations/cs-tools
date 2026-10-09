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

-- Planner-friendly internal-caller check in every RLS policy.
--
-- Problem: for an internal caller each policy is "app.is_internal = 'true' OR
-- ...", always true, but the planner cannot know that when it plans
-- (current_setting() is opaque to cardinality estimation, so the comparison
-- gets the default 0.5% selectivity). Measured on the real-data copy:
-- 2,046 estimated vs 409,296 actual work_item rows, which pushed queries
-- into nested loops with hundreds of thousands of per-row index probes.
--
-- Fix: wrap the comparison in a scalar sub-select,
--   (SELECT current_setting('app.is_internal', true) = 'true')
-- Postgres evaluates it once per query as an InitPlan instead of once per
-- row, and estimates it at a neutral 50% rather than 0.5%. No new role, no
-- change to who can see what: the predicate's truth value is identical.
--
-- Rewrites every existing policy that contains the old form. Idempotent:
-- old_expr is a literal substring of the wrapped form, so a bare match on
-- old_expr would re-wrap an already-fixed policy into (SELECT (SELECT ...)).
-- pg_policies deparses the wrapped form as "( SELECT <old_expr>)" (note the
-- space), which differs from the "(SELECT ...)" written below, so both
-- spellings are folded back to old_expr first; only policies that still
-- contain old_expr after that fold are rewritten, and the rewrite starts
-- from the folded text.
DO $$
DECLARE
  old_expr CONSTANT TEXT := $e$(current_setting('app.is_internal'::text, true) = 'true'::text)$e$;
  new_expr CONSTANT TEXT := $e$(SELECT (current_setting('app.is_internal'::text, true) = 'true'::text))$e$;
  wrapped_pg CONSTANT TEXT := '( SELECT ' || old_expr || ')';
  wrapped_src CONSTANT TEXT := '(SELECT ' || old_expr || ')';
  r RECORD;
  using_sql TEXT;
  check_sql TEXT;
  folded_qual TEXT;
  folded_check TEXT;
BEGIN
  FOR r IN
    SELECT policyname, tablename, qual, with_check
    FROM pg_policies
    WHERE schemaname = current_schema()
  LOOP
    folded_qual := replace(replace(r.qual, wrapped_pg, old_expr), wrapped_src, old_expr);
    folded_check := replace(replace(r.with_check, wrapped_pg, old_expr), wrapped_src, old_expr);
    -- Skip policies with nothing to wrap, and policies where every
    -- occurrence is already wrapped (the fold is then a no-op on the count
    -- of unwrapped occurrences, i.e. nothing outside a wrapper remains).
    IF position(old_expr IN coalesce(r.qual, '')) = 0 AND position(old_expr IN coalesce(r.with_check, '')) = 0 THEN
      CONTINUE;
    END IF;
    IF replace(replace(coalesce(r.qual, ''), wrapped_pg, ''), wrapped_src, '') NOT LIKE '%' || old_expr || '%'
       AND replace(replace(coalesce(r.with_check, ''), wrapped_pg, ''), wrapped_src, '') NOT LIKE '%' || old_expr || '%' THEN
      CONTINUE;
    END IF;
    using_sql := CASE WHEN r.qual IS NOT NULL
      THEN format(' USING (%s)', replace(folded_qual, old_expr, new_expr)) ELSE '' END;
    check_sql := CASE WHEN r.with_check IS NOT NULL
      THEN format(' WITH CHECK (%s)', replace(folded_check, old_expr, new_expr)) ELSE '' END;
    EXECUTE format('ALTER POLICY %I ON %I%s%s', r.policyname, r.tablename, using_sql, check_sql);
  END LOOP;
END
$$;
