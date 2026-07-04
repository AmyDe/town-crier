package uk.towncrierapp.domain.auth

/**
 * A user's identity information from the authentication provider. [email]
 * may legitimately be blank — Sign in with Apple grants no email scope — so
 * callers must render it gracefully rather than treat blank as an error.
 */
public data class UserProfile(
    val userId: String,
    val email: String,
    val name: String?,
) {
    /** Derives the authentication method from the [userId] (JWT `sub` claim) prefix. */
    public val authMethod: AuthMethod
        get() =
            when {
                userId.startsWith("auth0|") -> AuthMethod.EMAIL_PASSWORD
                userId.startsWith("google-oauth2|") -> AuthMethod.GOOGLE
                userId.startsWith("apple|") -> AuthMethod.APPLE
                else -> AuthMethod.UNKNOWN
            }
}
