package uk.towncrierapp.data.auth

import com.auth0.android.result.Credentials

/** Hand-written fake for [CredentialsStore] — no real Keystore/Context involved. */
internal class FakeCredentialsStore(
    var hasValidCredentialsResult: Boolean = false,
    var credentialsResult: Result<Credentials> = Result.failure(CredentialsStoreException.Unrecoverable),
) : CredentialsStore {
    val saveCredentialsCalls: MutableList<Credentials> = mutableListOf()
    val clearCredentialsCalls: MutableList<Unit> = mutableListOf()

    override fun hasValidCredentials(): Boolean = hasValidCredentialsResult

    override suspend fun credentials(): Credentials = credentialsResult.getOrThrow()

    override fun saveCredentials(credentials: Credentials) {
        saveCredentialsCalls += credentials
    }

    override fun clearCredentials() {
        clearCredentialsCalls += Unit
    }
}
