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
		tt := tt
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
