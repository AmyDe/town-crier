package pgbackup

// Table is one table in the backup. OrderBy is its primary key so a dump is
// deterministic.
type Table struct {
	Name    string
	OrderBy string
}

// Tables lists the user-owned tables in restore load order (parents before
// children). Adding a table here also needs a SELECT grant in
// cmd/pgbootstrap/grants_backup.sql.
var Tables = []Table{
	{Name: "users", OrderBy: "user_id"},
	{Name: "watch_zones", OrderBy: "id"},
	{Name: "saved_applications", OrderBy: "user_id, application_uid"},
	{Name: "notifications", OrderBy: "id"},
	{Name: "notification_state", OrderBy: "user_id"},
	{Name: "device_registrations", OrderBy: "user_id, token"},
	{Name: "offer_codes", OrderBy: "code"},
	{Name: "offer_code_redemptions", OrderBy: "id"},
}

// Excluded lists the tables deliberately left out of the backup: public data
// or data derived from it, anonymous totals, short-lived operational state, and
// schema metadata.
// Every table in the schema must be in exactly one of Tables or Excluded.
var Excluded = []string{
	"applications",
	"apple_notifications",
	"application_event",
	"backfill_state",
	"delta_seen",
	"goose_db_version",
	"leases",
	"planit_call",
	"poll_control",
	"poll_event",
	"poll_oracle_diff",
	"poll_state",
	"poll_window",
	"poll_window_member",
	"recent_sweep_state",
	"seo_pages",
	"seo_redirects",
	"seo_state",
	"seo_town_applications",
	"seo_towns",
	"share_event_totals",
	"share_page_click_totals",
	"spatial_ref_sys",
}
