package digest

import "github.com/AmyDe/town-crier/api-go/internal/notifications"

// dedupApplicationKey identifies duplicate notification records for the same
// application within one digest email. Identity is (ApplicationUID,
// AuthorityID) — the same PlanIt uid under two different authorities is a
// coincidence, not a duplicate.
type dedupApplicationKey struct {
	applicationUID string
	authorityID    int
}

// dedupByApplication collapses notifications that share an (ApplicationUID,
// AuthorityID) to a single winning record, so each application renders
// exactly once in the digest email. Two records legitimately coexist per
// application for the in-app feed (a NewApplication and a later
// DecisionUpdate record), but the email must not render both.
//
// Preference is decided by preferNotification: DecisionUpdate beats
// NewApplication (it carries the decision stamp); among equal event types, a
// zone-attributed record beats a saved-only one; ties break on the latest
// CreatedAt. The winning record is kept whole and placed wherever its own
// WatchZoneID says — dedup never merges fields or borrows a zone from the
// record it discarded.
//
// The result preserves the input's first-appearance order per application, so
// callers that group by order of appearance (groupByZone) see a stable
// section sequence.
func dedupByApplication(notifs []notifications.DigestNotification) []notifications.DigestNotification {
	order := make([]dedupApplicationKey, 0, len(notifs))
	winners := make(map[dedupApplicationKey]notifications.DigestNotification, len(notifs))

	for _, n := range notifs {
		key := dedupApplicationKey{applicationUID: n.ApplicationUID, authorityID: n.AuthorityID}
		incumbent, seen := winners[key]
		if !seen {
			order = append(order, key)
			winners[key] = n
			continue
		}
		if preferNotification(n, incumbent) {
			winners[key] = n
		}
	}

	deduped := make([]notifications.DigestNotification, 0, len(order))
	for _, key := range order {
		deduped = append(deduped, winners[key])
	}
	return deduped
}

// preferNotification reports whether candidate should replace incumbent as
// the winning record for a duplicate (ApplicationUID, AuthorityID) pair.
func preferNotification(candidate, incumbent notifications.DigestNotification) bool {
	if candidateScore, incumbentScore := eventTypeScore(candidate.EventType), eventTypeScore(incumbent.EventType); candidateScore != incumbentScore {
		return candidateScore > incumbentScore
	}
	if candidateZoned, incumbentZoned := candidate.WatchZoneID != nil, incumbent.WatchZoneID != nil; candidateZoned != incumbentZoned {
		return candidateZoned
	}
	return candidate.CreatedAt.After(incumbent.CreatedAt)
}

// eventTypeScore ranks event types for dedup preference: DecisionUpdate
// outranks NewApplication because it carries the decision stamp the email
// must show when an application's outcome is already known.
func eventTypeScore(t notifications.EventType) int {
	if t == notifications.EventDecisionUpdate {
		return 1
	}
	return 0
}
