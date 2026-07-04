package uk.towncrierapp.domain.reviewprompt

/**
 * A positive in-app engagement signal that feeds the store review-prompt
 * policy (GH #628). Signals are deliberately scoped to genuine *value*
 * moments inside the app — a review ask is never delivered by push. Most
 * signals can trigger a fire evaluation; [Upgraded] is a pure score
 * contributor that nudges a borderline user toward a prompt but can never,
 * on its own, be the moment that triggers the ask. Port of iOS `ReviewSignal`.
 */
public sealed interface ReviewSignal {
    /** The user tapped through to the council planning portal. */
    public data object TappedPortal : ReviewSignal

    /**
     * The user opened an instant alert via its push/deep-link detail path
     * (push-tap only — distinct from browsing the list).
     */
    public data object OpenedAlert : ReviewSignal

    /**
     * The user saved/bookmarked an application. Only the 2nd and later saves
     * are fire-eligible; the first save is usually setup rather than delight.
     */
    public data object SavedApplication : ReviewSignal

    /** The app was foregrounded on a new distinct calendar day (loyalty). */
    public data object ActiveDay : ReviewSignal

    /**
     * The user upgraded to a paid tier. Contributes score but is never a
     * fire moment, and is latched so it counts at most once.
     */
    public data object Upgraded : ReviewSignal
}
