package uk.towncrierapp.domain.auth

/** How the user signed in, derived from the JWT `sub` claim prefix (see [UserProfile.authMethod]). */
public enum class AuthMethod {
    EMAIL_PASSWORD,
    GOOGLE,
    APPLE,
    UNKNOWN,
}
