package uk.towncrierapp.domain.applications

import kotlinx.coroutines.CancellationException
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.watchzones.WatchZoneId
import java.time.Clock
import java.time.Duration

/**
 * A 900s TTL read-through cache in front of [remote]'s first-page (param-less,
 * `cursor == null`) fetch, keyed by [WatchZoneId] alone — regardless of which
 * [ApplicationSortOrder]/[ApplicationFilter] produced it (epic #770 parity: the
 * cache is "the last known good snapshot for this zone", not sort/filter-
 * aware). A non-null `cursor` (a pagination continuation) always bypasses the
 * cache entirely, going straight to [remote].
 *
 * Failure handling: ANY [DomainError] from [remote] (not just
 * [DomainError.NetworkUnavailable]) falls back to a cached entry when one
 * exists, however stale — this is deliberately broader than "network errors
 * only", matching iOS `OfflineAwareRepository`. With no cached entry to fall
 * back to, the original error propagates unchanged (so a genuine offline
 * failure surfaces as [DomainError.NetworkUnavailable], not something
 * misleading). [detail] is a plain, uncached pass-through — only the zone
 * application list is cached. Port of iOS `OfflineAwareRepository` (GH#775).
 */
public class OfflineAwareRepository(
    private val remote: PlanningApplicationRepository,
    private val cache: ApplicationCacheStore,
    private val clock: Clock = Clock.systemUTC(),
) : PlanningApplicationRepository {
    override suspend fun applications(
        zoneId: WatchZoneId,
        sort: ApplicationSortOrder,
        filter: ApplicationFilter,
        cursor: String?,
    ): ApplicationPage {
        if (cursor != null) return remote.applications(zoneId, sort, filter, cursor)

        val cached = cache.get(zoneId)
        if (cached != null && isFresh(cached)) return cached.page

        return try {
            val page = remote.applications(zoneId, sort, filter, cursor = null)
            cache.put(zoneId, CachedApplicationPage(page, cachedAt = clock.instant()))
            page
        } catch (e: CancellationException) {
            throw e
        } catch (e: DomainError) {
            cached?.page ?: throw e
        }
    }

    override suspend fun detail(
        authority: String,
        name: String,
    ): PlanningApplication = remote.detail(authority, name)

    private fun isFresh(cached: CachedApplicationPage): Boolean =
        Duration.between(cached.cachedAt, clock.instant()) < TTL

    public companion object {
        public val TTL: Duration = Duration.ofSeconds(900)
    }
}
