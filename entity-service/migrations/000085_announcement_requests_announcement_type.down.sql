ALTER TABLE announcement_requests
    ADD COLUMN is_security_announcement BOOLEAN NOT NULL DEFAULT false;

UPDATE announcement_requests
    SET is_security_announcement = (announcement_type = 'SECURITY');

ALTER TABLE announcement_requests DROP COLUMN announcement_type;
