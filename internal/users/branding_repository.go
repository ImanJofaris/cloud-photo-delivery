package users

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const brandingColumns = `user_id, COALESCE(business_name, ''), COALESCE(logo_key, ''),
	COALESCE(profile_image_key, ''), COALESCE(primary_color, ''), COALESCE(secondary_color, ''),
	COALESCE(contact_email, ''), COALESCE(contact_phone, ''), COALESCE(website_url, ''), updated_at`

func scanBranding(row pgx.Row) (*Branding, error) {
	var b Branding
	err := row.Scan(&b.UserID, &b.BusinessName, &b.LogoKey, &b.ProfileImageKey, &b.PrimaryColor,
		&b.SecondaryColor, &b.ContactEmail, &b.ContactPhone, &b.WebsiteURL, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &b, nil
}

func (r *PostgresRepository) GetBranding(ctx context.Context, userID uuid.UUID) (*Branding, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+brandingColumns+` FROM tenant_branding WHERE user_id = $1`, userID)
	return scanBranding(row)
}

func (r *PostgresRepository) UpsertBranding(ctx context.Context, b Branding) (*Branding, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO tenant_branding (user_id, business_name, logo_key, profile_image_key, primary_color,
			secondary_color, contact_email, contact_phone, website_url, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		 ON CONFLICT (user_id) DO UPDATE SET
			business_name = EXCLUDED.business_name,
			logo_key = EXCLUDED.logo_key,
			profile_image_key = EXCLUDED.profile_image_key,
			primary_color = EXCLUDED.primary_color,
			secondary_color = EXCLUDED.secondary_color,
			contact_email = EXCLUDED.contact_email,
			contact_phone = EXCLUDED.contact_phone,
			website_url = EXCLUDED.website_url,
			updated_at = NOW()
		 RETURNING `+brandingColumns,
		b.UserID, nullString(b.BusinessName), nullString(b.LogoKey), nullString(b.ProfileImageKey),
		nullString(b.PrimaryColor), nullString(b.SecondaryColor), nullString(b.ContactEmail),
		nullString(b.ContactPhone), nullString(b.WebsiteURL))
	return scanBranding(row)
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
