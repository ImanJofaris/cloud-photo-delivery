//go:build integration

package users_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func setupBrandingDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := setupDB(t)
	mustExec(t, pool, `CREATE TABLE tenant_branding (
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
	)`)
	return pool
}

func TestBrandingRepository_UpsertAndGet(t *testing.T) {
	pool := setupBrandingDB(t)
	repo := users.NewRepository(pool)
	ctx := context.Background()

	u, err := repo.Create(ctx, "brand@example.com", "hash", "Booth")
	require.NoError(t, err)

	_, err = repo.GetBranding(ctx, u.ID)
	require.ErrorIs(t, err, users.ErrNotFound)

	saved, err := repo.UpsertBranding(ctx, users.Branding{
		UserID:       u.ID,
		BusinessName: "Booth Co",
		LogoKey:      "tenant/" + u.ID.String() + "/branding/logo/a.png",
		PrimaryColor: "#112233",
		ContactEmail: "hello@example.com",
	})
	require.NoError(t, err)
	require.Equal(t, "Booth Co", saved.BusinessName)
	require.NotZero(t, saved.UpdatedAt)

	got, err := repo.GetBranding(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, "#112233", got.PrimaryColor)
	require.Equal(t, "tenant/"+u.ID.String()+"/branding/logo/a.png", got.LogoKey)
	require.Empty(t, got.SecondaryColor)

	updated, err := repo.UpsertBranding(ctx, users.Branding{
		UserID:         u.ID,
		BusinessName:   "Renamed Booth",
		SecondaryColor: "#445566",
	})
	require.NoError(t, err)
	require.Equal(t, "Renamed Booth", updated.BusinessName)
	require.Empty(t, updated.LogoKey)
	require.Equal(t, "#445566", updated.SecondaryColor)
}

func TestBrandingRepository_TenantIsolation(t *testing.T) {
	pool := setupBrandingDB(t)
	repo := users.NewRepository(pool)
	ctx := context.Background()

	a, err := repo.Create(ctx, "a@example.com", "hash", "")
	require.NoError(t, err)
	b, err := repo.Create(ctx, "b@example.com", "hash", "")
	require.NoError(t, err)

	_, err = repo.UpsertBranding(ctx, users.Branding{UserID: a.ID, BusinessName: "A Booth"})
	require.NoError(t, err)

	_, err = repo.GetBranding(ctx, b.ID)
	require.ErrorIs(t, err, users.ErrNotFound)

	_, err = repo.UpsertBranding(ctx, users.Branding{UserID: b.ID, BusinessName: "B Booth"})
	require.NoError(t, err)

	gotA, err := repo.GetBranding(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, "A Booth", gotA.BusinessName)

	gotB, err := repo.GetBranding(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, "B Booth", gotB.BusinessName)

	_, err = repo.GetBranding(ctx, uuid.New())
	require.ErrorIs(t, err, users.ErrNotFound)
}
