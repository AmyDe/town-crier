package uk.towncrierapp.domain.reviewprompt

/**
 * Performs the real platform review request. Abstracted behind an interface
 * so the decision flow in `ReviewPromptTracker` (`:presentation`) stays
 * unit-testable — the real Play In-App Review call sits at the `:app`
 * composition-root edge, since it needs a foreground `Activity`. Port of iOS
 * `ReviewRequesting`.
 */
public fun interface ReviewRequester {
    /** Best-effort: implementations should swallow failures (no Play listing, no Activity, ...) rather than throw. */
    public suspend fun requestReview()
}
