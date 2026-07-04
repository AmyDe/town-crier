package uk.towncrierapp.data.auth

import com.auth0.android.authentication.AuthenticationException
import com.auth0.android.authentication.storage.CredentialsManagerException
import com.auth0.android.authentication.storage.SecureCredentialsManager
import com.auth0.android.result.Credentials

/**
 * Production [CredentialsStore]: a thin adapter over the real
 * `SecureCredentialsManager` (Keystore-backed). Deliberately not unit tested
 * — `SecureCredentialsManager` needs a real `Context`/Keystore and has an
 * `internal` SDK constructor (android-coding-standards testing.md: "what not
 * to test" — thin, delegation-only Android glue; verified on the emulator).
 * [isUnrecoverable] is the one piece of real logic here, classifying the
 * SDK's `CredentialsManagerException` (whose `Code` enum is internal to the
 * SDK module) via its exposed singleton constants.
 */
public class SecureCredentialsManagerStore(
    private val manager: SecureCredentialsManager,
) : CredentialsStore {
    override fun hasValidCredentials(): Boolean = manager.hasValidCredentials()

    override suspend fun credentials(): Credentials =
        try {
            manager.awaitCredentials()
        } catch (e: CredentialsManagerException) {
            throw if (isUnrecoverable(
                    e,
                )
            ) {
                CredentialsStoreException.Unrecoverable
            } else {
                CredentialsStoreException.Transient(e)
            }
        }

    override fun saveCredentials(credentials: Credentials): Unit = manager.saveCredentials(credentials)

    override fun clearCredentials(): Unit = manager.clearCredentials()

    private fun isUnrecoverable(error: CredentialsManagerException): Boolean =
        when {
            error == CredentialsManagerException.NO_REFRESH_TOKEN -> {
                true
            }

            error == CredentialsManagerException.NO_CREDENTIALS -> {
                true
            }

            error == CredentialsManagerException.RENEW_FAILED ||
                error == CredentialsManagerException.SSO_EXCHANGE_FAILED -> {
                val cause = error.cause as? AuthenticationException
                cause?.isInvalidRefreshToken == true || cause?.isRefreshTokenDeleted == true
            }

            else -> {
                false
            }
        }
}
