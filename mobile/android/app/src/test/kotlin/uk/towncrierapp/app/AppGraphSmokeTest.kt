package uk.towncrierapp.app

import com.auth0.android.result.Credentials
import okhttp3.Call
import org.junit.jupiter.api.Test
import uk.towncrierapp.data.auth.CredentialsStore
import uk.towncrierapp.data.auth.CurrentActivityProvider
import uk.towncrierapp.data.legal.LegalDocumentAssetReader
import uk.towncrierapp.domain.applications.FakeApplicationListPreferencesStore
import uk.towncrierapp.domain.onboarding.FakeOnboardingRepository
import uk.towncrierapp.domain.reviewprompt.FakeReviewPromptStore
import uk.towncrierapp.domain.settings.FakeAppearanceStore
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

    private fun fakeAndroidLeaves() =
        AndroidLeaves(
            credentialsStore = NoOpCredentialsStore,
            activityProvider = CurrentActivityProvider { null },
            tierCache = FakeSubscriptionTierCache(),
            applicationListPreferencesStore = FakeApplicationListPreferencesStore(),
            onboardingRepository = FakeOnboardingRepository(),
            appearanceStore = FakeAppearanceStore(),
            reviewPromptStore = FakeReviewPromptStore(),
            legalDocumentAssetReader =
                LegalDocumentAssetReader {
                    error(
                        "not used in the composition-root smoke test",
                    )
                },
        )

    @Test
    fun `constructs the composition root without throwing`() {
        val graph =
            AppGraph(
                baseUrl = "https://api-dev.towncrierapp.uk",
                auth0Tenant = Auth0Tenant(clientId = "client-id", domain = "towncrierapp.uk.auth0.com"),
                androidLeaves = fakeAndroidLeaves(),
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
        assertNotNull(graph.applicationCacheStore)
        assertNotNull(graph.planningApplicationRepository)
        assertNotNull(graph.offlineAwareApplicationRepository)
        assertNotNull(graph.savedApplicationRepository)
        assertNotNull(graph.notificationStateRepository)
        assertNotNull(graph.applicationListPreferencesStore)
        assertNotNull(graph.onboardingRepository)
        assertNotNull(graph.appearanceCoordinator)
        assertNotNull(graph.legalDocumentRepository)
        assertNotNull(graph.reviewPromptTracker)
    }

    @Test
    fun `deviceTokenRepository is null until 777 lands a real implementation`() {
        val graph =
            AppGraph(
                baseUrl = "https://api-dev.towncrierapp.uk",
                auth0Tenant = Auth0Tenant(clientId = "client-id", domain = "towncrierapp.uk.auth0.com"),
                androidLeaves = fakeAndroidLeaves(),
                currentVersion = "0.1.0",
                options = AppGraphOptions(callFactory = noOpCallFactory),
            )

        assertEquals(null, graph.deviceTokenRepository)
    }

    @Test
    fun `authAudience defaults to baseUrl when not overridden`() {
        val graph =
            AppGraph(
                baseUrl = "https://api-dev.towncrierapp.uk",
                auth0Tenant = Auth0Tenant(clientId = "client-id", domain = "towncrierapp.uk.auth0.com"),
                androidLeaves = fakeAndroidLeaves(),
                currentVersion = "0.1.0",
                options = AppGraphOptions(callFactory = noOpCallFactory),
            )

        assertEquals("https://api-dev.towncrierapp.uk", graph.authAudience)
    }

    @Test
    fun `authAudience overrides independently of baseUrl for the local-flavor dev-audience case`() {
        val graph =
            AppGraph(
                baseUrl = "http://10.0.2.2:8080",
                authAudience = "https://api-dev.towncrierapp.uk",
                auth0Tenant = Auth0Tenant(clientId = "client-id", domain = "towncrierapp.uk.auth0.com"),
                androidLeaves = fakeAndroidLeaves(),
                currentVersion = "0.1.0",
                options = AppGraphOptions(callFactory = noOpCallFactory),
            )

        assertEquals("http://10.0.2.2:8080", graph.baseUrl)
        assertEquals("https://api-dev.towncrierapp.uk", graph.authAudience)
    }
}
