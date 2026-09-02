ALTER TABLE alert_buffer DROP CONSTRAINT IF EXISTS alert_buffer_alert_number_key;
ALTER TABLE alert_buffer DROP COLUMN IF EXISTS alert_number;
DROP SEQUENCE IF EXISTS alert_number_seq;
