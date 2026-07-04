package uk.towncrierapp.app

import com.auth0.android.result.Credentials
import okhttp3.Call
import org.junit.jupiter.api.Test
import uk.towncrierapp.data.auth.CredentialsStore
import uk.towncrierapp.data.auth.CurrentActivityProvider
import uk.towncrierapp.domain.subscriptions.FakeSubscriptionTierCache
import kotlin.test.assertEquals
import kotlin.test.assertNotNull

/**
 * Because the Android leaves are constructor parameters, a plain JVM test
 * can construct the whole composition root — wiring drift fails here, in
 * `./gradlew test`, rather than on first launch (android-coding-standards
 * skill, architecture-and-modules.md). Every Android-touching leaf here is
 * an explicit fake — even the default `Call.Factory` — so this test never
 * depends on whether a *real* `OkHttpClient`/`Auth0` happens to tolerate the
 * AGP unit-test android.jar stub.
 */
class AppGraphSmokeTest {
    private object NoOpCredentialsStore : CredentialsStore {
        override fun hasValidCredentials(): Boolean = false

        override suspend fun credentials(): Credentials = error("not used in the composition-root smoke test")

        override fun saveCredentials(credentials: Credentials) = Unit

        override fun clearCredentials() = Unit
    }

    private val noOpCallFactory = Call.Factory { error("not used in the composition-root smoke test") }

    @Test
    fun `constructs the composition root without throwing`() {
        val graph =
            AppGraph(
                baseUrl = "https://api-dev.towncrierapp.uk",
                auth0Tenant = Auth0Tenant(clientId = "client-id", domain = "towncrierapp.uk.auth0.com"),
                androidLeaves =
                    AndroidLeaves(
                        credentialsStore = NoOpCredentialsStore,
                        activityProvider = CurrentActivityProvider { null },
                        tierCache = FakeSubscriptionTierCache(),
                    ),
                currentVersion = "0.1.0",
                options = AppGraphOptions(callFactory = noOpCallFactory),
            )

        assertEquals("https://api-dev.towncrierapp.uk", graph.baseUrl)
        assertNotNull(graph.authenticationService)
        assertNotNull(graph.userProfileRepository)
        assertNotNull(graph.versionConfigService)
        assertNotNull(graph.authCoordinator)
        assertNotNull(graph.watchZoneRepository)
        assertNotNull(graph.zonePreferencesRepository)
        assertNotNull(graph.postcodeGeocoder)
    }
}
