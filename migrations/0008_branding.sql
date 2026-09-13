-- +goose Up
CREATE TABLE tenant_branding (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    business_name VARCHAR(255),
    logo_key TEXT,
    profile_image_key TEXT,
    primary_color VARCHAR(9),
    secondary_color VARCHAR(9),
    contact_email VARCHAR(320),
    contact_phone VARCHAR(40),
    website_url VARCHAR(320),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS tenant_branding;
