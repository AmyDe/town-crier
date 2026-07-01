package platform

// AppleUniversalLinkAppID is the Apple App ID (TeamID.BundleID) that associates
// the share host with the iOS app for Universal Links, served in the AASA
// document (#738, Slice 3). It is composed from the canonical team and bundle
// constants — never the APNS_TEAM_ID / APPLE_BUNDLE_ID runtime overrides — so it
// cannot drift from the fixed contract shared with the iOS entitlement and the
// web AASA.
func AppleUniversalLinkAppID() string {
	return defaultAPNsTeamID + "." + defaultAppleBundleID
}
