-- +goose Up
UPDATE plans
SET limits = (limits - 'activeEvents') || jsonb_build_object(
    'events', COALESCE((limits ->> 'events')::int, (limits ->> 'activeEvents')::int, 0),
    'branding', id <> 'free',
    'originalDownloads', id <> 'free'
)
WHERE limits ? 'activeEvents';

-- +goose Down
UPDATE plans
SET limits = (limits - 'events' - 'branding' - 'originalDownloads') || jsonb_build_object(
    'activeEvents', COALESCE((limits ->> 'activeEvents')::int, (limits ->> 'events')::int, 0)
)
WHERE limits ? 'events';
