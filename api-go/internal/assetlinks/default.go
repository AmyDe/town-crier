package assetlinks

// devPackageName is the dev-flavor Android application id (mirrors iOS's
// dev/prod bundle-id split, epic #770).
const devPackageName = "uk.towncrierapp.mobile.dev"

// devDebugKeystoreFingerprint is the SHA-256 certificate fingerprint of the
// shared debug keystore (colon-separated uppercase hex, computed locally for
// GH#782), which signs local and CI dev-flavor builds.
const devDebugKeystoreFingerprint = "75:2F:87:AF:52:B6:4D:33:71:ED:77:2A:2A:1C:D9:7A:A4:67:9E:1A:17:F0:9F:FD:92:12:D6:55:92:FD:0E:07"

// DefaultPackages returns the fixed Digital Asset Links contract for the
// Android package flavors (GH#782), composed from fixed constants rather than
// runtime config — mirroring platform.AppleUniversalLinkAppID's approach for
// the equivalent Apple contract.
//
// The prod package, uk.towncrierapp.mobile, has no entry yet: the Play App
// Signing certificate fingerprint is unavailable until Play Console
// enrolment (#779). It is omitted entirely — rather than published with a
// placeholder that would fail verification — matching Routes' own rule of
// dropping any Package with no fingerprints. Once #779 lands a real
// fingerprint, add its entry here.
// TestDefaultPackages_OmitsProdPackageUntilPlayConsoleEnrolment pins this gap
// so it surfaces as a test to update rather than a silent omission.
func DefaultPackages() []Package {
	return []Package{
		{Name: devPackageName, Fingerprints: []string{devDebugKeystoreFingerprint}},
	}
}
