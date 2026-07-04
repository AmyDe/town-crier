package uk.towncrierapp.presentation.features.applicationlist

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import uk.towncrierapp.domain.applications.ApplicationCacheStore
import uk.towncrierapp.domain.applications.ApplicationFilter
import uk.towncrierapp.domain.applications.ApplicationListPreferencesStore
import uk.towncrierapp.domain.applications.ApplicationSortOrder
import uk.towncrierapp.domain.applications.NotificationStateRepository
import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.domain.applications.PlanningApplicationRepository
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.domain.watchzones.WatchZoneRepository

/**
 * Drives the Applications tab: zone chips, status/unread filter chips, sort
 * persistence, cursor-based infinite scroll, tap-to-read, and mark-all-read.
 * Port of iOS `ApplicationListViewModel` (GH#775).
 */
public class ApplicationListViewModel(
    private val applicationRepository: PlanningApplicationRepository,
    private val watchZoneRepository: WatchZoneRepository,
    private val notificationStateRepository: NotificationStateRepository,
    private val applicationCacheStore: ApplicationCacheStore,
    private val preferencesStore: ApplicationListPreferencesStore,
) : ViewModel() {
    private val _uiState = MutableStateFlow(ApplicationListUiState())
    public val uiState: StateFlow<ApplicationListUiState> = _uiState.asStateFlow()

    // Guards against the spurious refetch-on-every-tab-revisit (tc-hlbx) —
    // see SavedListViewModel's matching field doc for the full rationale.
    // markAllRead()/selectZone()/selectSort()/selectFilter() call
    // fetchFirstPage() directly and are unaffected by this guard, so their
    // explicit refresh behaviour (tc-cnme) is unchanged.
    private var hasLoadedSuccessfully = false

    public fun load() {
        if (hasLoadedSuccessfully) return
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                val zones = watchZoneRepository.zones()
                val persistedZoneId = preferencesStore.readLastSelectedZoneId()
                val selected = zones.firstOrNull { it.id == persistedZoneId }?.id ?: zones.firstOrNull()?.id
                val sort = preferencesStore.readSort() ?: ApplicationSortOrder.DEFAULT
                _uiState.update { it.copy(zones = zones, sort = sort, selectedZoneId = selected) }
                hasLoadedSuccessfully = true
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(isLoading = false, error = e) }
                return@launch
            }
            fetchFirstPage()
        }
    }

    public fun selectZone(zoneId: WatchZoneId) {
        viewModelScope.launch { preferencesStore.writeLastSelectedZoneId(zoneId) }
        _uiState.update { it.copy(selectedZoneId = zoneId, applications = emptyList(), nextCursor = null) }
        fetchFirstPage()
    }

    public fun selectSort(sort: ApplicationSortOrder) {
        viewModelScope.launch { preferencesStore.writeSort(sort) }
        _uiState.update { it.copy(sort = sort, applications = emptyList(), nextCursor = null) }
        fetchFirstPage()
    }

    /** Sets the active chip. [ApplicationFilter]'s shape makes status/unread mutual exclusivity automatic — this always replaces the whole filter. */
    public fun selectFilter(filter: ApplicationFilter) {
        _uiState.update { it.copy(filter = filter, applications = emptyList(), nextCursor = null) }
        fetchFirstPage()
    }

    /** Called by the Screen for each composed row index; prefetches the next page once within [PREFETCH_THRESHOLD] rows of the end. */
    public fun onItemVisible(index: Int) {
        val state = _uiState.value
        if (state.hasMore && !state.isLoadingMore && index >= state.applications.size - PREFETCH_THRESHOLD) {
            loadMore()
        }
    }

    /** Optimistically clears [application]'s unread state, then fires-and-forgets the mark-read request (errors swallowed). */
    @Suppress("SwallowedException")
    // Fire-and-forget by design (GH#775): the optimistic local clear already
    // reflects the user's intent; a failed background sync isn't worth an
    // error banner the user didn't ask to see.
    public fun markAsRead(application: PlanningApplication) {
        if (application.latestUnreadEvent == null) return
        _uiState.update { state ->
            state.copy(
                applications =
                    state.applications.map { row ->
                        if (row.id == application.id) row.copy(latestUnreadEvent = null) else row
                    },
            )
        }
        viewModelScope.launch {
            try {
                notificationStateRepository.markRead(listOf(application.id))
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                // Swallowed — see the function doc.
            }
        }
    }

    /** Marks every application read server-side, invalidates every zone's offline cache, then refetches the active zone. */
    public fun markAllRead() {
        viewModelScope.launch {
            try {
                notificationStateRepository.markAllRead()
                applicationCacheStore.invalidateAll()
                fetchFirstPage()
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(error = e) }
            }
        }
    }

    private fun fetchFirstPage() {
        val zoneId = _uiState.value.selectedZoneId
        if (zoneId == null) {
            _uiState.update { it.copy(isLoading = false) }
            return
        }
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                val state = _uiState.value
                val page = applicationRepository.applications(zoneId, state.sort, state.filter)
                _uiState.update {
                    it.copy(
                        applications = page.applications,
                        nextCursor = page.nextCursor,
                        isLoading = false,
                    )
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(isLoading = false, error = e) }
            }
        }
    }

    private fun loadMore() {
        val state = _uiState.value
        val zoneId = state.selectedZoneId ?: return
        val cursor = state.nextCursor ?: return
        viewModelScope.launch {
            _uiState.update { it.copy(isLoadingMore = true) }
            try {
                val page = applicationRepository.applications(zoneId, state.sort, state.filter, cursor)
                _uiState.update {
                    it.copy(
                        applications = it.applications + page.applications,
                        nextCursor = page.nextCursor,
                        isLoadingMore = false,
                    )
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(isLoadingMore = false, error = e) }
            }
        }
    }

    public companion object {
        public const val PREFETCH_THRESHOLD: Int = 10
    }
}
