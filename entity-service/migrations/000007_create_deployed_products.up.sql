CREATE TABLE IF NOT EXISTS deployed_products (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id       UUID        NOT NULL REFERENCES deployments(id),
    product_id          UUID        NOT NULL REFERENCES products(id),
    product_version_id  UUID        REFERENCES product_versions(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_deployed_products_deployment_id      ON deployed_products (deployment_id);
CREATE INDEX IF NOT EXISTS idx_deployed_products_product_id         ON deployed_products (product_id);
CREATE INDEX IF NOT EXISTS idx_deployed_products_product_version_id ON deployed_products (product_version_id);

-- Prevent duplicate versioned associations; NULLs are distinct under a standard unique constraint
-- so a separate partial index handles the version-less case.
CREATE UNIQUE INDEX IF NOT EXISTS uq_deployed_products_versioned
    ON deployed_products (deployment_id, product_id, product_version_id)
    WHERE product_version_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_deployed_products_no_version
    ON deployed_products (deployment_id, product_id)
    WHERE product_version_id IS NULL;

CREATE OR REPLACE FUNCTION check_deployed_product_version()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.product_version_id IS NOT NULL THEN
    -- Version supplied: product must be software class and version must belong to it.
    IF NOT EXISTS (
      SELECT 1
      FROM product_versions pv
      JOIN products p ON pv.product_id = p.id
      WHERE pv.id   = NEW.product_version_id
        AND p.id    = NEW.product_id
        AND p.class = 'software'
    ) THEN
      RAISE EXCEPTION
        'Invalid product_version_id %: either product_id % is not a software product, or the version does not belong to this product.',
        NEW.product_version_id, NEW.product_id;
    END IF;
  ELSE
    -- No version supplied: software products must always specify a version.
    IF EXISTS (
      SELECT 1 FROM products WHERE id = NEW.product_id AND class = 'software'
    ) THEN
      RAISE EXCEPTION
        'product_version_id is required for software product_id %.',
        NEW.product_id;
    END IF;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_deployed_product_version
  BEFORE INSERT OR UPDATE ON deployed_products
  FOR EACH ROW EXECUTE FUNCTION check_deployed_product_version();
