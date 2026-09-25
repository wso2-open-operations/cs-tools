-- Replaces announcement_requests' own bespoke is_security_announcement
-- boolean with the same announcement_type_enum (GENERAL/SECURITY) the
-- "announcement" table already uses (migration 000084_announcement_add_type)
-- -- one shared vocabulary for "is this a security announcement" across the
-- whole flow (pre-publish request and published case) instead of two
-- independently-typed columns doing the same job.
ALTER TABLE announcement_requests
    ADD COLUMN announcement_type announcement_type_enum NOT NULL DEFAULT 'GENERAL';

UPDATE announcement_requests
    SET announcement_type = CASE
        WHEN is_security_announcement THEN 'SECURITY'::announcement_type_enum
        ELSE 'GENERAL'::announcement_type_enum
    END;

ALTER TABLE announcement_requests DROP COLUMN is_security_announcement;
