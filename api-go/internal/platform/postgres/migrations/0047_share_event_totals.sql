-- +goose Up

CREATE TABLE IF NOT EXISTS share_event_totals (
    day         date   NOT NULL,
    destination text   NOT NULL,
    total       bigint NOT NULL,
    PRIMARY KEY (day, destination)
);

-- +goose Down

DROP TABLE IF EXISTS share_event_totals;
