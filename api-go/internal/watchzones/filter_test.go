package watchzones

import "testing"

func TestIsValidFilterKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"fewer_notifications", string(FilterKeyFewerNotifications), true},
		{"house_builder", string(FilterKeyHouseBuilder), true},
		{"new_homes", string(FilterKeyNewHomes), true},
		{"extensions_alterations", string(FilterKeyExtensionsAlterations), true},
		{"loft_extension", string(FilterKeyLoftExtension), true},
		{"kitchen_extension", string(FilterKeyKitchenExtension), true},
		{"hmo_house_shares", string(FilterKeyHMOHouseShares), true},
		{"bogus key", "not_a_real_filter", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsValidFilterKey(tt.key); got != tt.want {
				t.Errorf("IsValidFilterKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestFilterCatalog_Completeness(t *testing.T) {
	t.Parallel()

	if len(filterCatalog) != 7 {
		t.Fatalf("filterCatalog has %d entries, want 7", len(filterCatalog))
	}

	for key, def := range filterCatalog {
		if def.DisplayName == "" {
			t.Errorf("filterCatalog[%q].DisplayName is empty", key)
		}
		if def.Description == "" {
			t.Errorf("filterCatalog[%q].Description is empty", key)
		}
	}
}

func TestFilterCatalog_FewerNotificationsHasNoExpression(t *testing.T) {
	t.Parallel()

	def, ok := filterCatalog[FilterKeyFewerNotifications]
	if !ok {
		t.Fatalf("filterCatalog missing %q", FilterKeyFewerNotifications)
	}
	if def.Expression != "" {
		t.Errorf("FilterKeyFewerNotifications.Expression = %q, want empty (app_type gate only)", def.Expression)
	}
}

func TestFilterDefinitionFor(t *testing.T) {
	t.Parallel()

	def, ok := FilterDefinitionFor(FilterKeyHouseBuilder)
	if !ok {
		t.Fatalf("FilterDefinitionFor(%q) = _, false, want true", FilterKeyHouseBuilder)
	}
	if def.DisplayName != filterCatalog[FilterKeyHouseBuilder].DisplayName {
		t.Errorf("FilterDefinitionFor(%q) DisplayName = %q, want %q",
			FilterKeyHouseBuilder, def.DisplayName, filterCatalog[FilterKeyHouseBuilder].DisplayName)
	}

	if _, ok := FilterDefinitionFor(FilterKey("not_a_real_filter")); ok {
		t.Error("FilterDefinitionFor(unknown key) = _, true, want false")
	}
}

func TestIsExcludedAppType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		appType string
		want    bool
	}{
		{"Trees is excluded", "Trees", true},
		{"Telecoms is excluded", "Telecoms", true},
		{"Full is not excluded", "Full", false},
		{"empty string is not excluded", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsExcludedAppType(tt.appType); got != tt.want {
				t.Errorf("IsExcludedAppType(%q) = %v, want %v", tt.appType, got, tt.want)
			}
		})
	}
}

func TestFilterCatalog_KeywordFiltersHaveExpressions(t *testing.T) {
	t.Parallel()

	keywordKeys := []FilterKey{
		FilterKeyHouseBuilder,
		FilterKeyNewHomes,
		FilterKeyExtensionsAlterations,
		FilterKeyLoftExtension,
		FilterKeyKitchenExtension,
		FilterKeyHMOHouseShares,
	}

	for _, key := range keywordKeys {
		def, ok := filterCatalog[key]
		if !ok {
			t.Fatalf("filterCatalog missing %q", key)
		}
		if def.Expression == "" {
			t.Errorf("filterCatalog[%q].Expression is empty, want a to_tsquery expression", key)
		}
	}
}

// TestFilterCatalogOrder_MatchesFilterCatalog guards against filterCatalog
// and filterCatalogOrder drifting apart. A key added to filterCatalog alone
// would build and pass every other test, but silently never appear in GET
// /v1/watch-zones/filter-catalog (catalog_handler.go skips any
// filterCatalogOrder entry missing from filterCatalog, but nothing catches
// the reverse) -- defeating GH#1104/GH#1090's "adding a filter is a Go-only
// deploy" premise.
func TestFilterCatalogOrder_MatchesFilterCatalog(t *testing.T) {
	t.Parallel()

	if len(filterCatalogOrder) != len(filterCatalog) {
		t.Fatalf("filterCatalogOrder has %d entries, filterCatalog has %d -- they must match",
			len(filterCatalogOrder), len(filterCatalog))
	}

	seen := make(map[FilterKey]bool, len(filterCatalogOrder))
	for _, key := range filterCatalogOrder {
		if _, ok := filterCatalog[key]; !ok {
			t.Errorf("filterCatalogOrder contains %q, which is not in filterCatalog", key)
		}
		if seen[key] {
			t.Errorf("filterCatalogOrder contains %q more than once", key)
		}
		seen[key] = true
	}

	for key := range filterCatalog {
		if !seen[key] {
			t.Errorf("filterCatalog contains %q, which is missing from filterCatalogOrder", key)
		}
	}
}
