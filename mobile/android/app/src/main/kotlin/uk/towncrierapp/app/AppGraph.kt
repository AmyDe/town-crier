package uk.towncrierapp.app

import com.auth0.android.Auth0
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import okhttp3.Call
import okhttp3.OkHttpClient
import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.OkHttpTransport
import uk.towncrierapp.data.applications.ApiNotificationStateRepository
import uk.towncrierapp.data.applications.ApiPlanningApplicationRepository
import uk.towncrierapp.data.applications.ApiSavedApplicationRepository
import uk.towncrierapp.data.applications.InMemoryApplicationCacheStore
import uk.towncrierapp.data.auth.Auth0AuthenticationService
import uk.towncrierapp.data.auth.Auth0Config
import uk.towncrierapp.data.auth.CredentialsStore
import uk.towncrierapp.data.auth.CurrentActivityProvider
import uk.towncrierapp.data.auth.SessionCache
import uk.towncrierapp.data.legal.LegalDocumentAssetReader
import uk.towncrierapp.data.legal.LegalDocumentLoader
import uk.towncrierapp.data.profile.ApiUserProfileRepository
import uk.towncrierapp.data.versionconfig.ApiVersionConfigService
import uk.towncrierapp.data.watchzones.ApiPostcodeGeocoder
import uk.towncrierapp.data.watchzones.ApiWatchZoneRepository
import uk.towncrierapp.data.watchzones.ApiZonePreferencesRepository
import uk.towncrierapp.domain.applications.ApplicationCacheStore
import uk.towncrierapp.domain.applications.ApplicationListPreferencesStore
import uk.towncrierapp.domain.applications.NotificationStateRepository
import uk.towncrierapp.domain.applications.OfflineAwareRepository
import uk.towncrierapp.domain.applications.PlanningApplicationRepository
import uk.towncrierapp.domain.applications.SavedApplicationRepository
import uk.towncrierapp.domain.auth.AuthenticationService
import uk.towncrierapp.domain.devicetoken.DeviceTokenRepository
import uk.towncrierapp.domain.legal.LegalDocumentRepository
import uk.towncrierapp.domain.onboarding.OnboardingRepository
import uk.towncrierapp.domain.profile.UserProfileRepository
import uk.towncrierapp.domain.reviewprompt.ReviewPromptStore
import uk.towncrierapp.domain.settings.AppearanceStore
import uk.towncrierapp.domain.subscriptions.SubscriptionTierCache
import uk.towncrierapp.domain.versionconfig.VersionConfigService
import uk.towncrierapp.domain.watchzones.PostcodeGeocoder
import uk.towncrierapp.domain.watchzones.WatchZoneRepository
import uk.towncrierapp.domain.watchzones.ZonePreferencesRepository
import uk.towncrierapp.presentation.appearance.AppearanceCoordinator
import uk.towncrierapp.presentation.auth.AuthCoordinator
import uk.towncrierapp.presentation.reviewprompt.ReviewPromptTracker
import java.time.Clock

/** Auth0 tenant identity — same across build flavors (epic #770 D4); only the audience differs, via [AppGraph.authAudience]. */
public data class Auth0Tenant(
    val clientId: String,
    val domain: String,
)

/**
 * The leaves that genuinely need a real `Context` —
 * `TownCrierApplication` builds them (`SecureCredentialsManagerStore` over a
 * real `SecureCredentialsManager`, an `Application.ActivityLifecycleCallbacks`
 * tracker, a `DataStoreSubscriptionTierCache`, a `DataStoreApplicationListPreferencesStore`,
 * a `DataStoreOnboardingRepository`, a `DataStoreAppearanceStore`, a
 * `DataStoreReviewPromptStore`, an `AssetManager`-backed [LegalDocumentAssetReader])
 * and hands them to the otherwise Context-free [AppGraph].
 */
public class AndroidLeaves(
    public val credentialsStore: CredentialsStore,
    public val activityProvider: CurrentActivityProvider,
    public val tierCache: SubscriptionTierCache,
    public val applicationListPreferencesStore: ApplicationListPreferencesStore,
    public val onboardingRepository: OnboardingRepository,
    public val appearanceStore: AppearanceStore,
    public val reviewPromptStore: ReviewPromptStore,
    public val legalDocumentAssetReader: LegalDocumentAssetReader,
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
    // Defaults to baseUrl (dev/prod: audience == the flavor's own API), but is
    // independently overridable — the `local` flavor talks to a local API
    // (baseUrl = http://10.0.2.2:8080) while still requesting dev-audience
    // JWTs, because the local API is configured to validate that audience.
    public val authAudience: String = baseUrl,
) {
    private val transport = OkHttpTransport(options.callFactory)

    // Surfaced publicly (not just used to build authenticationService below)
    // because the "Rate the App" settings row also needs a foreground
    // Activity for its own, differently-failure-handled Play In-App Review
    // attempt — see PlayReviewRequester.kt's `requestReviewOrOpenStoreListing`.
    public val activityProvider: CurrentActivityProvider = androidLeaves.activityProvider

    // dev and prod share one Auth0 Native application; only the audience (and
    // therefore the access-token claims) differ per flavor (epic #770 D4).
    public val authenticationService: AuthenticationService =
        Auth0AuthenticationService(
            config = Auth0Config(clientId = auth0Tenant.clientId, domain = auth0Tenant.domain, audience = authAudience),
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

    public val watchZoneRepository: WatchZoneRepository = ApiWatchZoneRepository(apiClient)

    public val authCoordinator: AuthCoordinator =
        AuthCoordinator(authenticationService, userProfileRepository, androidLeaves.tierCache, watchZoneRepository)

    public val zonePreferencesRepository: ZonePreferencesRepository = ApiZonePreferencesRepository(apiClient)

    public val postcodeGeocoder: PostcodeGeocoder = ApiPostcodeGeocoder(apiClient)

    // The offline cache seam (GH#775 / tc-cnme): a single InMemoryApplicationCacheStore
    // instance backs BOTH the (currently dormant — see offlineAwareApplicationRepository
    // below) OfflineAwareRepository decorator AND the explicit
    // invalidate(zoneId)/invalidateAll() hooks a watch-zone edit and a
    // mark-all-read fire — same object, so an invalidation is always visible.
    public val applicationCacheStore: ApplicationCacheStore = InMemoryApplicationCacheStore()

    // The interactive Applications tab/detail screen's every fetch — first
    // page included — goes straight to the network, matching iOS parity: on
    // iOS every current sort is server-driven, so ApplicationListViewModel's
    // `fetchPage` always calls `offlineRepository.fetchApplicationsPage`
    // (network-only) and the OfflineAwareRepository-cached
    // `fetchApplications(for:)` path is DORMANT plumbing "kept generic
    // should a future client-only sort ever be added" (iOS
    // ApplicationListViewModel+Pagination.swift) — it is never reached in
    // practice. Routing the interactive screen through the cache instead
    // would silently freeze filter/sort changes for the 900s TTL, since the
    // cache key is zone-only (verified live on-device, GH#775).
    public val planningApplicationRepository: PlanningApplicationRepository =
        ApiPlanningApplicationRepository(apiClient)

    // Built and available (mirrors iOS's own dormant `OfflineAwareRepository`
    // — kept for a hypothetical future non-server-sorted mode) but not
    // currently consumed by any ViewModel; see planningApplicationRepository's
    // doc above.
    public val offlineAwareApplicationRepository: OfflineAwareRepository =
        OfflineAwareRepository(
            remote = planningApplicationRepository,
            cache = applicationCacheStore,
            clock = options.clock,
        )

    public val savedApplicationRepository: SavedApplicationRepository = ApiSavedApplicationRepository(apiClient)

    public val notificationStateRepository: NotificationStateRepository = ApiNotificationStateRepository(apiClient)

    public val applicationListPreferencesStore: ApplicationListPreferencesStore =
        androidLeaves.applicationListPreferencesStore

    public val onboardingRepository: OnboardingRepository = androidLeaves.onboardingRepository

    public val appearanceCoordinator: AppearanceCoordinator = AppearanceCoordinator(androidLeaves.appearanceStore)

    public val legalDocumentRepository: LegalDocumentRepository =
        LegalDocumentLoader(androidLeaves.legalDocumentAssetReader)

    public val reviewPromptTracker: ReviewPromptTracker =
        ReviewPromptTracker(
            store = androidLeaves.reviewPromptStore,
            requester = PlayReviewRequester(activityProvider),
            clock = options.clock,
        )

    // #777 hasn't landed a real device-token registration yet — `null` here
    // means Settings' sign-out/deletion device-token removal is a true no-op
    // in production today; wiring a real implementation needs no other change.
    public val deviceTokenRepository: DeviceTokenRepository? = null
}
