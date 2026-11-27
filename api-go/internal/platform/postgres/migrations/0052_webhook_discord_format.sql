-- +goose Up
ALTER TABLE webhook_endpoints
    DROP CONSTRAINT IF EXISTS webhook_endpoints_format_check,
    ADD CONSTRAINT webhook_endpoints_format_check CHECK (format IN ('json', 'slack', 'discord'));

-- +goose Down
UPDATE webhook_endpoints SET format = 'json' WHERE format = 'discord';

ALTER TABLE webhook_endpoints
    DROP CONSTRAINT IF EXISTS webhook_endpoints_format_check,
    ADD CONSTRAINT webhook_endpoints_format_check CHECK (format IN ('json', 'slack'));
