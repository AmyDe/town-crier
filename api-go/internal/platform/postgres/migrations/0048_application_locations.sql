-- +goose Up

CREATE TABLE application_locations (
    authority_code  text NOT NULL,
    planit_name     text NOT NULL,
    uid             text,
    area_id         integer,
    location        geography(Point, 4326) NOT NULL,
    app_state       text,
    start_date      date
) WITH (fillfactor = 90);

CREATE TABLE read_path_control (
    id          boolean     PRIMARY KEY DEFAULT true CHECK (id),
    thin_sync   boolean     NOT NULL DEFAULT false,
    thin_list   boolean     NOT NULL DEFAULT false,
    thin_map    boolean     NOT NULL DEFAULT false,
    reason      text        NOT NULL DEFAULT '',
    updated_at  timestamptz NOT NULL
);

-- +goose StatementBegin
CREATE FUNCTION application_locations_sync() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    sync_on boolean;
BEGIN
    SELECT thin_sync INTO sync_on FROM read_path_control WHERE id;
    IF NOT COALESCE(sync_on, false) THEN
        RETURN NULL;
    END IF;

    -- A failed thin write must never abort the poll upsert that fired this trigger.
    BEGIN
        IF TG_OP = 'DELETE'
           OR (TG_OP = 'UPDATE' AND (OLD.authority_code IS DISTINCT FROM NEW.authority_code
                                     OR OLD.planit_name IS DISTINCT FROM NEW.planit_name)) THEN
            DELETE FROM application_locations
            WHERE authority_code = OLD.authority_code AND planit_name = OLD.planit_name;
        END IF;

        IF TG_OP <> 'DELETE' THEN
            IF NEW.location IS NULL THEN
                DELETE FROM application_locations
                WHERE authority_code = NEW.authority_code AND planit_name = NEW.planit_name;
            ELSE
                INSERT INTO application_locations
                    (authority_code, planit_name, uid, area_id, location, app_state, start_date)
                VALUES
                    (NEW.authority_code, NEW.planit_name, NEW.uid, NEW.area_id, NEW.location, NEW.app_state, NEW.start_date)
                ON CONFLICT (authority_code, planit_name) DO UPDATE SET
                    uid = EXCLUDED.uid,
                    area_id = EXCLUDED.area_id,
                    location = EXCLUDED.location,
                    app_state = EXCLUDED.app_state,
                    start_date = EXCLUDED.start_date;
            END IF;
        END IF;
    EXCEPTION WHEN OTHERS THEN
        RAISE WARNING 'application_locations sync failed: % (%)', SQLERRM, SQLSTATE;
    END;

    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER application_locations_sync_update
    AFTER UPDATE ON applications
    FOR EACH ROW
    WHEN (OLD.authority_code IS DISTINCT FROM NEW.authority_code
          OR OLD.planit_name IS DISTINCT FROM NEW.planit_name
          OR OLD.uid IS DISTINCT FROM NEW.uid
          OR OLD.area_id IS DISTINCT FROM NEW.area_id
          OR OLD.location IS DISTINCT FROM NEW.location
          OR OLD.app_state IS DISTINCT FROM NEW.app_state
          OR OLD.start_date IS DISTINCT FROM NEW.start_date)
    EXECUTE FUNCTION application_locations_sync();

CREATE TRIGGER application_locations_sync_insert_delete
    AFTER INSERT OR DELETE ON applications
    FOR EACH ROW
    EXECUTE FUNCTION application_locations_sync();

-- +goose Down

DROP TRIGGER IF EXISTS application_locations_sync_insert_delete ON applications;
DROP TRIGGER IF EXISTS application_locations_sync_update ON applications;
DROP FUNCTION IF EXISTS application_locations_sync();
DROP TABLE IF EXISTS read_path_control;
DROP TABLE IF EXISTS application_locations;
