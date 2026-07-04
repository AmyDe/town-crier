package uk.towncrierapp.app

import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.OkHttpTransport
import uk.towncrierapp.data.auth.Auth0AuthenticationService
import uk.towncrierapp.data.auth.Auth0Config
import uk.towncrierapp.data.auth.CredentialsStore
import uk.towncrierapp.data.auth.CurrentActivityProvider
import uk.towncrierapp.data.auth.SessionCache
import uk.towncrierapp.data.profile.ApiUserProfileRepository
import uk.towncrierapp.data.versionconfig.ApiVersionConfigService
import uk.towncrierapp.domain.auth.AuthenticationService
import uk.towncrierapp.domain.profile.UserProfileRepository
import uk.towncrierapp.domain.subscriptions.SubscriptionTierCache
import uk.towncrierapp.domain.versionconfig.VersionConfigService
import uk.towncrierapp.presentation.auth.AuthCoordinator
import com.auth0.android.Auth0
import java.time.Clock
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import okhttp3.Call
import okhttp3.OkHttpClient

/**
 * Town Crier's composition root: the single place `:app` hand-wires the
 * dependency graph from `:domain` ports to `:data` implementations, via
 * manual constructor injection (epic #770 — no DI framework, no Hilt/Koin).
 * Android-touching leaves come in through the constructor as their domain
 * interfaces, so this class itself stays a pure-JVM type — which is what
 * lets [AppGraphSmokeTest] construct it in a plain JVM test.
 *
 * [credentialsStore] (a `SecureCredentialsManagerStore` over a real
 * `SecureCredentialsManager`), [activityProvider] (an `Application.
 * ActivityLifecycleCallbacks`-backed tracker), and [tierCache] (a
 * `DataStoreSubscriptionTierCache`) are the three leaves that genuinely need
 * a `Context` — `TownCrierApplication` builds them and passes them in here.
 */
public class AppGraph(
    public val baseUrl: String,
    auth0ClientId: String,
    auth0Domain: String,
    credentialsStore: CredentialsStore,
    activityProvider: CurrentActivityProvider,
    tierCache: SubscriptionTierCache,
    public val currentVersion: String,
    enableDebugLogging: Boolean = false,
    callFactory: Call.Factory = OkHttpClient.Builder().build(),
    scope: CoroutineScope = CoroutineScope(SupervisorJob() + Dispatchers.Default),
    clock: Clock = Clock.systemUTC(),
) {
    private val transport = OkHttpTransport(callFactory)

    // Audience = the flavor's own API base URL (epic #770 D4) — dev and prod
    // share one Auth0 Native application; only the audience (and therefore
    // the access-token claims) differ per flavor.
    public val authenticationService: AuthenticationService =
        Auth0AuthenticationService(
            config = Auth0Config(clientId = auth0ClientId, domain = auth0Domain, audience = baseUrl),
            credentialsStore = credentialsStore,
            activityProvider = activityProvider,
            auth0 = lazy { Auth0.getInstance(auth0ClientId, auth0Domain) },
            sessionCache = SessionCache(scope),
            clock = clock,
        )

    private val apiClient =
        ApiClient(
            baseUrl = baseUrl,
            transport = transport,
            authService = authenticationService,
            enableDebugLogging = enableDebugLogging,
        )

    public val userProfileRepository: UserProfileRepository = ApiUserProfileRepository(apiClient)

    // Anonymous, pre-login — shares the same raw transport but never goes
    // through ApiClient (which always requires a session first).
    public val versionConfigService: VersionConfigService = ApiVersionConfigService(baseUrl, transport)

    public val authCoordinator: AuthCoordinator = AuthCoordinator(authenticationService, userProfileRepository, tierCache)
}
