package uk.towncrierapp.data.auth

/** Auth0 tenant configuration for one build flavor. Both flavors share [clientId] and [domain]; only [audience] differs (epic #770 D4). */
public data class Auth0Config(
    val clientId: String,
    val domain: String,
    val audience: String,
) {
    init {
        require(audience.isNotEmpty()) { "Auth0Config.audience must not be empty" }
    }

    internal companion object {
        /** Scope requested at login — `offline_access` is what makes the refresh token flow work at all. */
        const val SCOPE = "openid profile email offline_access"
    }
}
