package uk.towncrierapp.app

import android.app.Application
import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.preferencesDataStore
import com.auth0.android.Auth0
import com.auth0.android.authentication.storage.SecureCredentialsManager
import com.auth0.android.authentication.storage.SharedPreferencesStorage
import uk.towncrierapp.data.applications.DataStoreApplicationListPreferencesStore
import uk.towncrierapp.data.auth.SecureCredentialsManagerStore
import uk.towncrierapp.data.legal.LegalDocumentAssetReader
import uk.towncrierapp.data.onboarding.DataStoreOnboardingRepository
import uk.towncrierapp.data.reviewprompt.DataStoreReviewPromptStore
import uk.towncrierapp.data.settings.DataStoreAppearanceStore
import uk.towncrierapp.data.subscriptions.DataStoreSubscriptionTierCache
import uk.towncrierapp.mobile.BuildConfig

/** Auth0 tenant — same across flavors; only the audience (`BuildConfig.AUTH0_AUDIENCE`) differs per flavor (epic #770 D4). */
internal const val AUTH0_CLIENT_ID = "2HHUYWnJ3q37a6Elv0cqyFVGbGIbqx34"
internal const val AUTH0_DOMAIN = "towncrierapp.uk.auth0.com"

private val Context.tierPreferencesDataStore: DataStore<Preferences> by preferencesDataStore(
    name = "town_crier_preferences",
)

/**
 * Constructs [AppGraph] — the one place `:app` builds the Android-touching
 * leaves that need a real `Context` (Keystore-backed credentials storage,
 * DataStore, the current-Activity tracker) before handing them to the
 * otherwise Context-free composition root (android-coding-standards skill,
 * architecture-and-modules.md).
 */
public class TownCrierApplication : Application() {
    public lateinit var appGraph: AppGraph
        private set

    override fun onCreate() {
        super.onCreate()

        val activityTracker = CurrentActivityTracker()
        registerActivityLifecycleCallbacks(activityTracker)

        val auth0 = Auth0.getInstance(AUTH0_CLIENT_ID, AUTH0_DOMAIN)
        val credentialsManager = SecureCredentialsManager(this, auth0, SharedPreferencesStorage(this))
        val credentialsStore = SecureCredentialsManagerStore(credentialsManager)

        val tierCache = DataStoreSubscriptionTierCache(tierPreferencesDataStore)
        // Same shared "town_crier_preferences" DataStore file as tierCache —
        // DataStore Preferences is designed for several unrelated keys in one
        // file; a second file per feature isn't warranted (GH#775 / tc-7ttz).
        val applicationListPreferencesStore = DataStoreApplicationListPreferencesStore(tierPreferencesDataStore)
        val onboardingRepository = DataStoreOnboardingRepository(tierPreferencesDataStore)
        val appearanceStore = DataStoreAppearanceStore(tierPreferencesDataStore)
        val reviewPromptStore = DataStoreReviewPromptStore(tierPreferencesDataStore)
        val legalDocumentAssetReader =
            LegalDocumentAssetReader { assetPath -> assets.open(assetPath).bufferedReader().use { it.readText() } }

        appGraph =
            AppGraph(
                baseUrl = BuildConfig.API_BASE_URL,
                authAudience = BuildConfig.AUTH0_AUDIENCE,
                auth0Tenant = Auth0Tenant(clientId = AUTH0_CLIENT_ID, domain = AUTH0_DOMAIN),
                androidLeaves =
                    AndroidLeaves(
                        credentialsStore = credentialsStore,
                        activityProvider = activityTracker,
                        tierCache = tierCache,
                        applicationListPreferencesStore = applicationListPreferencesStore,
                        onboardingRepository = onboardingRepository,
                        settingsLeaves =
                            SettingsLeaves(
                                appearanceStore = appearanceStore,
                                reviewPromptStore = reviewPromptStore,
                                legalDocumentAssetReader = legalDocumentAssetReader,
                            ),
                    ),
                currentVersion = BuildConfig.VERSION_NAME,
                options = AppGraphOptions(enableDebugLogging = BuildConfig.DEBUG),
            )
    }
}
