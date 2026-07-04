package uk.towncrierapp.presentation.features.applicationlist

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.applications.ApplicationFilter
import uk.towncrierapp.domain.applications.ApplicationSortOrder
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.FakeApplicationCacheStore
import uk.towncrierapp.domain.applications.FakeApplicationListPreferencesStore
import uk.towncrierapp.domain.applications.FakeNotificationStateRepository
import uk.towncrierapp.domain.applications.FakePlanningApplicationRepository
import uk.towncrierapp.domain.applications.aLatestUnreadEvent
import uk.towncrierapp.domain.applications.aPlanningApplication
import uk.towncrierapp.domain.applications.anApplicationPage
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.watchzones.FakeWatchZoneRepository
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.domain.watchzones.aWatchZone
import uk.towncrierapp.presentation.MainDispatcherExtension
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** Port of the iOS `ApplicationListViewModel*Tests` suites (GH#775). */
@ExtendWith(MainDispatcherExtension::class)
class ApplicationListViewModelTest {
    private fun makeSut(
        applicationRepository: FakePlanningApplicationRepository = FakePlanningApplicationRepository(),
        watchZoneRepository: FakeWatchZoneRepository = FakeWatchZoneRepository(mutableListOf(aWatchZone())),
        notificationStateRepository: FakeNotificationStateRepository = FakeNotificationStateRepository(),
        applicationCacheStore: FakeApplicationCacheStore = FakeApplicationCacheStore(),
        preferencesStore: FakeApplicationListPreferencesStore = FakeApplicationListPreferencesStore(),
    ) = ApplicationListViewModel(
        applicationRepository,
        watchZoneRepository,
        notificationStateRepository,
        applicationCacheStore,
        preferencesStore,
    )

    @Test
    fun `the default sort is recent-activity when nothing is persisted`() {
        val viewModel = makeSut()

        viewModel.load()

        assertEquals(ApplicationSortOrder.RECENT_ACTIVITY, viewModel.uiState.value.sort)
    }

    @Test
    fun `load restores a persisted sort`() {
        val preferences = FakeApplicationListPreferencesStore(storedSort = ApplicationSortOrder.OLDEST)
        val viewModel = makeSut(preferencesStore = preferences)

        viewModel.load()

        assertEquals(ApplicationSortOrder.OLDEST, viewModel.uiState.value.sort)
    }

    @Test
    fun `selecting a sort persists it and refetches the first page`() {
        val preferences = FakeApplicationListPreferencesStore()
        val applicationRepository = FakePlanningApplicationRepository()
        val viewModel = makeSut(applicationRepository = applicationRepository, preferencesStore = preferences)
        viewModel.load()

        viewModel.selectSort(ApplicationSortOrder.NEWEST)

        assertEquals(ApplicationSortOrder.NEWEST, preferences.storedSort)
        assertEquals(ApplicationSortOrder.NEWEST, applicationRepository.applicationsCalls.last().sort)
    }

    @Test
    fun `load restores a persisted last-selected zone when it is still in the zone list`() {
        val zoneA = aWatchZone(id = WatchZoneId("wz-a"))
        val zoneB = aWatchZone(id = WatchZoneId("wz-b"))
        val watchZoneRepository = FakeWatchZoneRepository(mutableListOf(zoneA, zoneB))
        val preferences = FakeApplicationListPreferencesStore(storedZoneId = WatchZoneId("wz-b"))
        val viewModel = makeSut(watchZoneRepository = watchZoneRepository, preferencesStore = preferences)

        viewModel.load()

        assertEquals(WatchZoneId("wz-b"), viewModel.uiState.value.selectedZoneId)
    }

    @Test
    fun `selecting a filter replaces it entirely — status and unread are mutually exclusive`() {
        val applicationRepository = FakePlanningApplicationRepository()
        val viewModel = makeSut(applicationRepository = applicationRepository)
        viewModel.load()

        viewModel.selectFilter(ApplicationFilter.Status(ApplicationStatus.Permitted))
        assertEquals(ApplicationFilter.Status(ApplicationStatus.Permitted), viewModel.uiState.value.filter)
        assertIs<ApplicationFilter.Status>(applicationRepository.applicationsCalls.last().filter)

        viewModel.selectFilter(ApplicationFilter.Unread)
        assertEquals(ApplicationFilter.Unread, viewModel.uiState.value.filter)
        assertEquals(ApplicationFilter.Unread, applicationRepository.applicationsCalls.last().filter)
    }

    @Test
    fun `availableSorts hides distance when no zone is active`() {
        val watchZoneRepository = FakeWatchZoneRepository(mutableListOf())
        val viewModel = makeSut(watchZoneRepository = watchZoneRepository)

        viewModel.load()

        assertFalse(
            viewModel.uiState.value.availableSorts
                .contains(ApplicationSortOrder.DISTANCE),
        )
    }

    @Test
    fun `availableSorts includes distance once a zone is active`() {
        val viewModel = makeSut()

        viewModel.load()

        assertTrue(
            viewModel.uiState.value.availableSorts
                .contains(ApplicationSortOrder.DISTANCE),
        )
    }

    @Test
    fun `onItemVisible prefetches the next page once within 10 rows of the end`() {
        val applications = List(20) { aPlanningApplication() }
        val applicationRepository =
            FakePlanningApplicationRepository().apply {
                applicationsResult = anApplicationPage(applications = applications, nextCursor = "cursor-2")
            }
        val viewModel = makeSut(applicationRepository = applicationRepository)
        viewModel.load()
        applicationRepository.applicationsResult = anApplicationPage(applications = emptyList(), nextCursor = null)

        viewModel.onItemVisible(applications.size - 11)
        assertEquals(1, applicationRepository.applicationsCalls.size)

        viewModel.onItemVisible(applications.size - 10)
        assertEquals(2, applicationRepository.applicationsCalls.size)
        assertEquals("cursor-2", applicationRepository.applicationsCalls.last().cursor)
    }

    @Test
    fun `unreadCount counts only currently-loaded rows with an unread event`() {
        val unread = aPlanningApplication(latestUnreadEvent = aLatestUnreadEvent())
        val read =
            aPlanningApplication(
                id =
                    uk.towncrierapp.domain.applications
                        .aPlanningApplicationId(name = "24/0002"),
            )
        val applicationRepository =
            FakePlanningApplicationRepository().apply { applicationsResult = anApplicationPage(listOf(unread, read)) }
        val viewModel = makeSut(applicationRepository = applicationRepository)

        viewModel.load()

        assertEquals(1, viewModel.uiState.value.unreadCount)
    }

    @Test
    fun `markAsRead optimistically clears the row's unread state and fires-and-forgets the request`() {
        val unread = aPlanningApplication(latestUnreadEvent = aLatestUnreadEvent())
        val applicationRepository =
            FakePlanningApplicationRepository().apply {
                applicationsResult =
                    anApplicationPage(listOf(unread))
            }
        val notificationStateRepository = FakeNotificationStateRepository()
        val viewModel =
            makeSut(
                applicationRepository = applicationRepository,
                notificationStateRepository = notificationStateRepository,
            )
        viewModel.load()

        viewModel.markAsRead(unread)

        assertNull(
            viewModel.uiState.value.applications
                .single()
                .latestUnreadEvent,
        )
        assertEquals(listOf(unread.id), notificationStateRepository.markReadCalls.single())
    }

    @Test
    fun `markAsRead swallows a mark-read failure — the optimistic clear stands and no error surfaces`() {
        val unread = aPlanningApplication(latestUnreadEvent = aLatestUnreadEvent())
        val applicationRepository =
            FakePlanningApplicationRepository().apply {
                applicationsResult =
                    anApplicationPage(listOf(unread))
            }
        val notificationStateRepository =
            FakeNotificationStateRepository().apply {
                markReadFailWith =
                    DomainError.NetworkUnavailable
            }
        val viewModel =
            makeSut(
                applicationRepository = applicationRepository,
                notificationStateRepository = notificationStateRepository,
            )
        viewModel.load()

        viewModel.markAsRead(unread)

        assertNull(
            viewModel.uiState.value.applications
                .single()
                .latestUnreadEvent,
        )
        assertNull(viewModel.uiState.value.error)
    }

    @Test
    fun `markAllRead invalidates every zone's cache then refetches the active zone`() {
        val applicationRepository = FakePlanningApplicationRepository()
        val applicationCacheStore = FakeApplicationCacheStore()
        val notificationStateRepository = FakeNotificationStateRepository()
        val viewModel =
            makeSut(
                applicationRepository = applicationRepository,
                applicationCacheStore = applicationCacheStore,
                notificationStateRepository = notificationStateRepository,
            )
        viewModel.load()
        val callsBeforeMarkAllRead = applicationRepository.applicationsCalls.size

        viewModel.markAllRead()

        assertEquals(1, notificationStateRepository.markAllReadCallCount)
        assertEquals(1, applicationCacheStore.invalidateAllCallCount)
        assertTrue(applicationRepository.applicationsCalls.size > callsBeforeMarkAllRead)
    }
}
