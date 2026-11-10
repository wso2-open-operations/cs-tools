-- A custom Team Schedule tag belongs to the family of whoever added it (a CRE
-- ABT lead's tag is CRE's), and only that family's rosters offer it. Tags a
-- rota admin added already carry their family; a tag a team lead added was
-- stored shared (family NULL), so every family saw it.
--
-- This gives each such shared custom tag its creator's family: the family of
-- the rota admin role they hold, or of the rota team they lead. A tag whose
-- creator spans more than one family, or none (a portal admin, someone who
-- has since left), is left shared rather than guessed. Built-in tags (created
-- by migration) and the moves to another team are never touched.
--
-- Safe to re-run: it only fills a family that is still NULL.
WITH creator_family AS (
    SELECT k.id, array_agg(DISTINCT f.family) AS families
      FROM team_schedule_absence_kind k
      JOIN "user" u ON lower(u.email) = lower(k.created_by)
      JOIN LATERAL (
            SELECT CASE ro.name
                     WHEN 'cre_rota_admin' THEN 'CRE'
                     WHEN 'sre_rota_admin' THEN 'SRE'
                     WHEN 'sme_rota_admin' THEN 'SME'
                   END AS family
              FROM user_role ur
              JOIN role ro ON ro.id = ur.role_id
             WHERE ur.user_id = u.id
               AND ro.name IN ('cre_rota_admin', 'sre_rota_admin', 'sme_rota_admin')
            UNION
            SELECT CASE WHEN lower(t.type) LIKE 'sre%' THEN 'SRE'
                        WHEN lower(t.type) LIKE 'sme%' THEN 'SME'
                        ELSE 'CRE' END
              FROM team_member tm
              JOIN team t ON t.id = tm.team_id
             WHERE tm.user_id = u.id
               AND tm.role IN ('lead', 'americas_team_lead')
               -- Same team test as the service's LeadFamiliesFor
               -- (rosteredTeamWhere in schedule_repo.go): keep the two in step.
               AND t.type IS NOT NULL AND lower(t.type) LIKE ANY (ARRAY['cre%', 'sre%', 'sme%'])
               AND lower(t.type) NOT LIKE '%leadership%' AND lower(COALESCE(t.name, '')) NOT LIKE '%leadership%'
               AND (lower(t.type) LIKE '%-abt'
                    OR EXISTS (SELECT 1 FROM team_schedule_rota rr WHERE lower(rr.team_type) = lower(t.type) AND rr.is_active)
                    OR EXISTS (SELECT 1 FROM team_schedule_team_default_shift dd WHERE dd.team_key = t.key)
                    OR EXISTS (SELECT 1 FROM team_schedule_absence_kind kk WHERE lower(kk.moves_to_team_key) = lower(t.key))
                    OR EXISTS (SELECT 1 FROM team_schedule_assignment aa WHERE aa.team_key = t.key)
                    OR EXISTS (SELECT 1 FROM team_schedule_absence bb WHERE bb.team_key = t.key))
           ) f ON TRUE
     WHERE k.family IS NULL
       AND k.created_by IS DISTINCT FROM 'migration'
       AND k.moves_to_team_key IS NULL
     GROUP BY k.id
)
UPDATE team_schedule_absence_kind k
   SET family = c.families[1]::team_schedule_shift_family_enum
  FROM creator_family c
 WHERE k.id = c.id
   AND cardinality(c.families) = 1
   AND c.families[1] IS NOT NULL
   AND k.family IS NULL;
