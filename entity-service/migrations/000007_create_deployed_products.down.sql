DROP TRIGGER IF EXISTS trg_deployed_product_version ON deployed_products;
DROP FUNCTION IF EXISTS check_deployed_product_version();
DROP INDEX IF EXISTS uq_deployed_products_versioned;
DROP INDEX IF EXISTS uq_deployed_products_no_version;
DROP TABLE IF EXISTS deployed_products;
