CREATE TYPE product_class_enum AS ENUM ('software', 'service');

CREATE TABLE IF NOT EXISTS products (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    class      product_class_enum NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_products_name_class UNIQUE (name, class)
);

CREATE INDEX IF NOT EXISTS idx_products_class      ON products (class);
CREATE INDEX IF NOT EXISTS idx_products_name_class ON products (name, class);
