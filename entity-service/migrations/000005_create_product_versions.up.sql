CREATE TYPE current_support_status_enum AS ENUM (
    'discontinued',
    'extended',
    'deprecated',
    'available'
);

CREATE TABLE IF NOT EXISTS product_versions (
    id                                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id                          UUID        NOT NULL REFERENCES products(id),
    version                             TEXT        NOT NULL,
    current_support_status              current_support_status_enum NOT NULL,
    release_date                        DATE        NOT NULL,
    support_eol_date                    DATE,
    earliest_possible_support_eol_date  DATE,
    created_at                          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_product_version UNIQUE (product_id, version)
);

CREATE OR REPLACE FUNCTION check_product_version_class()
RETURNS TRIGGER AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM products
        WHERE id = NEW.product_id AND class = 'software'
    ) THEN
        RAISE EXCEPTION 'product_versions can only be created for software class products. product_id % is not a software product.', NEW.product_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_product_versions_software_only
    BEFORE INSERT OR UPDATE ON product_versions
    FOR EACH ROW EXECUTE FUNCTION check_product_version_class();

CREATE INDEX IF NOT EXISTS idx_product_versions_product_id      ON product_versions (product_id);
CREATE INDEX IF NOT EXISTS idx_product_versions_support_status  ON product_versions (current_support_status);
CREATE INDEX IF NOT EXISTS idx_product_versions_release_date    ON product_versions (release_date);
CREATE INDEX IF NOT EXISTS idx_product_versions_eol_date        ON product_versions (support_eol_date);
CREATE INDEX IF NOT EXISTS idx_product_versions_product_release ON product_versions (product_id, release_date DESC);
