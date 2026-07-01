package platform

import "testing"

// TestAppleUniversalLinkAppID pins the composed Apple App ID served in the AASA
// document (#738, Slice 3). It is a fixed contract shared with the iOS
// entitlement and the web AASA, so the exact value must not drift.
func TestAppleUniversalLinkAppID(t *testing.T) {
	t.Parallel()

	const want = "4574VQ7N2X.uk.towncrierapp.mobile"
	if got := AppleUniversalLinkAppID(); got != want {
		t.Errorf("AppleUniversalLinkAppID() = %q, want %q", got, want)
	}
}
