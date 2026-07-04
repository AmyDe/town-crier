package uk.towncrierapp.data.auth

import android.app.Activity

/**
 * Supplies the current foreground [Activity] to [Auth0AuthenticationService]
 * at the moment `login()`/`logout()` run — Auth0's `WebAuthProvider` needs an
 * Activity context to launch Custom Tabs, but the `:domain` port
 * (`AuthenticationService.login()`) deliberately takes none, so it can stay
 * a plain-Kotlin interface. `:app` wires the real implementation by tracking
 * `Application.ActivityLifecycleCallbacks` — never held longer than needed,
 * never stored across process states.
 */
public fun interface CurrentActivityProvider {
    public fun currentActivity(): Activity?
}
