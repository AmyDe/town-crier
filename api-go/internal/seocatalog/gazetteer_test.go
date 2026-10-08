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
	if len(g.Hash) != 64 {
		t.Errorf("hash length = %d, want 64 hex chars", len(g.Hash))
	}
}

func TestParseGazetteer_HashTracksContent(t *testing.T) {
	t.Parallel()
	a, err := ParseGazetteer([]byte(`[{"slug":"a","name":"A","lat":1,"lng":2,"authorityId":3,"population":4}]`))
	if err != nil {
		t.Fatalf("parse a: %v", err)
	}
	b, err := ParseGazetteer([]byte(`[{"slug":"b","name":"B","lat":1,"lng":2,"authorityId":3,"population":4}]`))
	if err != nil {
		t.Fatalf("parse b: %v", err)
	}
	if a.Hash == b.Hash {
		t.Error("different content produced the same hash")
	}
	if a.Towns[0].AuthorityID != 3 || a.Towns[0].Population != 4 {
		t.Errorf("town decoded as %+v", a.Towns[0])
	}
}

func TestParseGazetteer_RejectsMalformedJSON(t *testing.T) {
	t.Parallel()
	if _, err := ParseGazetteer([]byte(`{`)); err == nil {
		t.Error("want an error for malformed JSON")
	}
}
