package uk.towncrierapp.app

import com.auth0.android.Auth0
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import okhttp3.Call
import okhttp3.OkHttpClient
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
import java.time.Clock

/** Auth0 tenant identity — same for both build flavors (epic #770 D4); only the audience differs, and that's [AppGraph.baseUrl]. */
public data class Auth0Tenant(
    val clientId: String,
    val domain: String,
)

/**
 * The three leaves that genuinely need a real `Context` —
 * `TownCrierApplication` builds them (`SecureCredentialsManagerStore` over a
 * real `SecureCredentialsManager`, an `Application.ActivityLifecycleCallbacks`
 * tracker, a `DataStoreSubscriptionTierCache`) and hands them to the
 * otherwise Context-free [AppGraph].
 */
public class AndroidLeaves(
    public val credentialsStore: CredentialsStore,
    public val activityProvider: CurrentActivityProvider,
    public val tierCache: SubscriptionTierCache,
)

/** Rarely-overridden technical knobs, grouped so [AppGraph]'s own constructor stays short. */
public class AppGraphOptions(
    public val enableDebugLogging: Boolean = false,
    public val callFactory: Call.Factory = OkHttpClient.Builder().build(),
    public val scope: CoroutineScope = CoroutineScope(SupervisorJob() + Dispatchers.Default),
    public val clock: Clock = Clock.systemUTC(),
)

/**
 * Town Crier's composition root: the single place `:app` hand-wires the
 * dependency graph from `:domain` ports to `:data` implementations, via
 * manual constructor injection (epic #770 — no DI framework, no Hilt/Koin).
 * Android-touching leaves come in through the constructor (via
 * [AndroidLeaves]) as their domain interfaces, so this class itself stays a
 * pure-JVM type — which is what lets [AppGraphSmokeTest] construct it in a
 * plain JVM test.
 */
public class AppGraph(
    public val baseUrl: String,
    auth0Tenant: Auth0Tenant,
    androidLeaves: AndroidLeaves,
    public val currentVersion: String,
    options: AppGraphOptions = AppGraphOptions(),
) {
    private val transport = OkHttpTransport(options.callFactory)

    // Audience = the flavor's own API base URL (epic #770 D4) — dev and prod
    // share one Auth0 Native application; only the audience (and therefore
    // the access-token claims) differ per flavor.
    public val authenticationService: AuthenticationService =
        Auth0AuthenticationService(
            config = Auth0Config(clientId = auth0Tenant.clientId, domain = auth0Tenant.domain, audience = baseUrl),
            credentialsStore = androidLeaves.credentialsStore,
            activityProvider = androidLeaves.activityProvider,
            auth0 = lazy { Auth0.getInstance(auth0Tenant.clientId, auth0Tenant.domain) },
            sessionCache = SessionCache(options.scope),
            clock = options.clock,
        )

    private val apiClient =
        ApiClient(
            baseUrl = baseUrl,
            transport = transport,
            authService = authenticationService,
            enableDebugLogging = options.enableDebugLogging,
        )

    public val userProfileRepository: UserProfileRepository = ApiUserProfileRepository(apiClient)

    // Anonymous, pre-login — shares the same raw transport but never goes
    // through ApiClient (which always requires a session first).
    public val versionConfigService: VersionConfigService = ApiVersionConfigService(baseUrl, transport)

    public val authCoordinator: AuthCoordinator =
        AuthCoordinator(authenticationService, userProfileRepository, androidLeaves.tierCache)
}
