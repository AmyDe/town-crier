-- +goose Up

CREATE TABLE IF NOT EXISTS share_page_click_totals (
    day            date   NOT NULL,
    authority_slug text   NOT NULL,
    ref            text   NOT NULL,
    source_group   text   NOT NULL CHECK (source_group IN ('facebook', 'seo', 'other', 'none')),
    clicks         bigint NOT NULL CHECK (clicks > 0),
    PRIMARY KEY (day, authority_slug, ref, source_group)
);

-- +goose Down

DROP TABLE IF EXISTS share_page_click_totals;
