package assetlinks

import "testing"

// TestDefaultPackages_IncludesDebugKeystoreFingerprintForDevPackage pins the
// dev-flavor entry to the debug keystore's SHA-256 fingerprint (computed
// locally for GH#782) so a locally-built dev APK verifies against the
// deployed document.
func TestDefaultPackages_IncludesDebugKeystoreFingerprintForDevPackage(t *testing.T) {
	t.Parallel()

	const wantFingerprint = "75:2F:87:AF:52:B6:4D:33:71:ED:77:2A:2A:1C:D9:7A:A4:67:9E:1A:17:F0:9F:FD:92:12:D6:55:92:FD:0E:07"

	packages := DefaultPackages()

	var dev *Package
	for i := range packages {
		if packages[i].Name == "uk.towncrierapp.mobile.dev" {
			dev = &packages[i]
		}
	}
	if dev == nil {
		t.Fatalf("DefaultPackages() = %+v, want an entry for uk.towncrierapp.mobile.dev", packages)
	}
	if len(dev.Fingerprints) != 1 || dev.Fingerprints[0] != wantFingerprint {
		t.Errorf("uk.towncrierapp.mobile.dev fingerprints = %v, want [%q]", dev.Fingerprints, wantFingerprint)
	}
}

// TestDefaultPackages_OmitsProdPackageUntilPlayConsoleEnrolment documents the
// deliberate gap: uk.towncrierapp.mobile has no Play App Signing certificate
// fingerprint until Play Console enrolment (#779). This test exists so that
// bead lands loudly (as a failing test to update) rather than silently once a
// real fingerprint is available.
func TestDefaultPackages_OmitsProdPackageUntilPlayConsoleEnrolment(t *testing.T) {
	t.Parallel()

	for _, p := range DefaultPackages() {
		if p.Name == "uk.towncrierapp.mobile" {
			t.Fatalf("uk.towncrierapp.mobile must be absent until #779 lands a real fingerprint; got %+v", p)
		}
	}
}
