-- +goose Up
UPDATE plans
SET limits = (limits - 'activeEvents') || jsonb_build_object(
    'events', COALESCE((limits ->> 'activeEvents')::int, 0),
    'branding', id <> 'free',
    'originalDownloads', id <> 'free'
);

-- +goose Down
UPDATE plans
SET limits = (limits - 'events' - 'branding' - 'originalDownloads') || jsonb_build_object(
    'activeEvents', COALESCE((limits ->> 'events')::int, 0)
);
