package uk.towncrierapp.presentation.sharing

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

/**
 * Holds an inbound App Link until the user is authenticated, then hands it
 * off exactly once (GH#782). A link tapped while signed out must NOT trigger
 * a fetch - the by-slug read is anonymous, but the app experience is authed
 * day-1 (no signed-out detail view) - so it is held here until
 * [onAuthenticationChanged] reports `true`, then surfaced via [readyLink] for
 * the composition layer to dispatch and [consume]. One instance lives for
 * the process lifetime, owned by the composition root (`AppGraph`).
 */
public class PendingLinkHolder {
    private var isAuthenticated = false
    private var heldLink: DeepLink? = null

    private val _readyLink = MutableStateFlow<DeepLink?>(null)
    public val readyLink: StateFlow<DeepLink?> = _readyLink.asStateFlow()

    /** Called when a new App Link arrives - cold start (`MainActivity.onCreate`) or a running-instance re-delivery (`onNewIntent`). */
    public fun linkReceived(link: DeepLink) {
        if (isAuthenticated) {
            _readyLink.value = link
        } else {
            heldLink = link
        }
    }

    /** Called on every auth-state transition; dispatches a held link the moment sign-in completes. */
    public fun onAuthenticationChanged(authenticated: Boolean) {
        isAuthenticated = authenticated
        if (authenticated) {
            heldLink?.let {
                _readyLink.value = it
                heldLink = null
            }
        }
    }

    /** Clears the ready link once the caller has dispatched it - a link is delivered exactly once. */
    public fun consume() {
        _readyLink.value = null
    }
}
