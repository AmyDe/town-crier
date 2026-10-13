package seocatalog

import "testing"

func TestEmbeddedGazetteer_ParsesAllTowns(t *testing.T) {
	t.Parallel()
	g, err := EmbeddedGazetteer()
	if err != nil {
		t.Fatalf("EmbeddedGazetteer: %v", err)
	}
	if len(g.Towns) != 1550 {
		t.Errorf("towns = %d, want 1550", len(g.Towns))
	}
}

func TestParseGazetteer_DecodesFields(t *testing.T) {
	t.Parallel()
	g, err := ParseGazetteer([]byte(`[{"slug":"a","name":"A","lat":1,"lng":2,"authorityId":3,"population":4}]`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if g.Towns[0].AuthorityID != 3 || g.Towns[0].Population != 4 {
		t.Errorf("town decoded as %+v", g.Towns[0])
	}
}

func TestParseGazetteer_RejectsMalformedJSON(t *testing.T) {
	t.Parallel()
	if _, err := ParseGazetteer([]byte(`{`)); err == nil {
		t.Error("want an error for malformed JSON")
	}
}
