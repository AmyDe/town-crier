package uk.towncrierapp.data.auth

import com.auth0.android.result.Credentials

/**
 * Seam over the Auth0 Android SDK's `SecureCredentialsManager` (which has an
 * `internal` constructor and needs a real `Context`, so it cannot itself be
 * subclassed or faked). [Auth0AuthenticationService]'s testable logic
 * (aud-mismatch wipe, renewability, refresh unrecoverable-vs-transient) is
 * written entirely against this interface; [SecureCredentialsManagerStore]
 * is the thin, untested-by-design Android adapter, and `FakeCredentialsStore`
 * (test source set) stands in for unit tests.
 */
public interface CredentialsStore {
    /** Whether stored credentials are valid — including refresh-token renewability (the real SDK's `hasValidCredentials()` already folds this in; tc-funq). */
    public fun hasValidCredentials(): Boolean

    /** Returns the current credentials, renewing via the refresh token if the access token has expired. Throws [CredentialsStoreException]. */
    public suspend fun credentials(): Credentials

    public fun saveCredentials(credentials: Credentials)

    public fun clearCredentials()
}

/**
 * Normalised failure classification for [CredentialsStore.credentials] — the
 * real adapter is responsible for inspecting the Auth0 SDK's
 * `CredentialsManagerException` (whose `Code` enum is internal to the SDK
 * module and not visible here) and mapping it to one of these two cases.
 */
internal sealed class CredentialsStoreException(
    override val cause: Throwable? = null,
) : Exception(cause) {
    /** No refresh token present, or it is invalid/revoked — re-authentication is required. */
    internal object Unrecoverable : CredentialsStoreException()

    /** A network or Keystore hiccup — transient, must NOT wipe stored credentials. */
    internal class Transient(
        cause: Throwable,
    ) : CredentialsStoreException(cause)
}
