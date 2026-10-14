package seocatalog

import "testing"

func TestIsSameNameAsAuthority(t *testing.T) {
	t.Parallel()
	tests := []struct {
		authority string
		town      string
		want      bool
	}{
		{"Wrexham", "wrexham", true},
		{"Hull", "kingston-upon-hull", false},
		{"Herefordshire", "hereford", false},
		{"Bristol, City of", "bristol", true},
		{"Wrexham / Wrecsam", "wrexham", true},
		{"Wrexham (Wrecsam)", "wrexham", true},
		{"King's Lynn and West Norfolk", "kings-lynn-and-west-norfolk", true},
		{"Cornwall", "truro", false},
	}
	for _, tt := range tests {
		t.Run(tt.authority+"/"+tt.town, func(t *testing.T) {
			t.Parallel()
			if got := isSameNameAsAuthority(tt.authority, tt.town); got != tt.want {
				t.Errorf("isSameNameAsAuthority(%q, %q) = %v, want %v", tt.authority, tt.town, got, tt.want)
			}
		})
	}
}
