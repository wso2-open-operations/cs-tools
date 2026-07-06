DROP TRIGGER IF EXISTS trg_product_versions_software_only ON product_versions;
DROP FUNCTION IF EXISTS check_product_version_class();
DROP TABLE IF EXISTS product_versions;
DROP TYPE IF EXISTS current_support_status_enum;
