package assetlinks

// devPackageName is the dev-flavor Android application id.
const devPackageName = "uk.towncrierapp.mobile.dev"

// devDebugKeystoreFingerprint is the SHA-256 certificate fingerprint of the
// shared debug keystore (colon-separated uppercase hex), which signs local and
// CI dev-flavor builds.
const devDebugKeystoreFingerprint = "75:2F:87:AF:52:B6:4D:33:71:ED:77:2A:2A:1C:D9:7A:A4:67:9E:1A:17:F0:9F:FD:92:12:D6:55:92:FD:0E:07"

// DefaultPackages returns the fixed Digital Asset Links contract for the
// Android package flavors. The prod package, uk.towncrierapp.mobile, is omitted
// until Play Console enrolment yields its signing fingerprint (#779), because a
// placeholder would fail verification.
func DefaultPackages() []Package {
	return []Package{
		{Name: devPackageName, Fingerprints: []string{devDebugKeystoreFingerprint}},
	}
}
