package uk.towncrierapp.presentation.features.map

import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.FakePlanningApplicationRepository
import uk.towncrierapp.domain.applications.FakeSavedApplicationRepository
import uk.towncrierapp.domain.applications.aPlanningApplication
import uk.towncrierapp.domain.applications.aSavedApplication
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.map.FakeMapPreferencesStore
import uk.towncrierapp.domain.map.MapViewport
import uk.towncrierapp.domain.map.aSingleMemberMapCluster
import uk.towncrierapp.domain.watchzones.FakeWatchZoneRepository
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.domain.watchzones.aWatchZone
import uk.towncrierapp.presentation.MainDispatcherExtension
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * [MapViewModel] load/zone-selection/status-filter/error behaviour. Port of
 * the iOS `MapViewModelTests`/`MapViewModelStatusFilterTests`/
 * `MapViewModelStaleZoneTests` suites (GH#776). Debounced camera-idle
 * coalescing and tap routing live in their own files (StandardTestDispatcher
 * / concurrency needs, see those files' docs).
 */
@ExtendWith(MainDispatcherExtension::class)
class MapViewModelTest {
    private fun makeSut(
        planningRepository: FakePlanningApplicationRepository = FakePlanningApplicationRepository(),
        watchZoneRepository: FakeWatchZoneRepository = FakeWatchZoneRepository(),
        savedApplicationRepository: FakeSavedApplicationRepository = FakeSavedApplicationRepository(),
        mapPreferencesStore: FakeMapPreferencesStore = FakeMapPreferencesStore(),
    ) = MapViewModel(planningRepository, watchZoneRepository, savedApplicationRepository, mapPreferencesStore)

    @Test
    fun `load resolves the previously-persisted zone over the first zone`() =
        runTest {
            val home = aWatchZone(id = WatchZoneId("wz-home"), name = "Home")
            val office = aWatchZone(id = WatchZoneId("wz-office"), name = "Office")
            val watchZoneRepository = FakeWatchZoneRepository(mutableListOf(home, office))
            val mapPreferencesStore = FakeMapPreferencesStore(storedZoneId = WatchZoneId("wz-office"))
            val sut = makeSut(watchZoneRepository = watchZoneRepository, mapPreferencesStore = mapPreferencesStore)

            sut.load()

            assertEquals(office, sut.uiState.value.selectedZone)
        }

    @Test
    fun `load falls back to the first zone when nothing was persisted`() =
        runTest {
            val home = aWatchZone(id = WatchZoneId("wz-home"))
            val watchZoneRepository = FakeWatchZoneRepository(mutableListOf(home))
            val sut = makeSut(watchZoneRepository = watchZoneRepository)

            sut.load()

            assertEquals(home, sut.uiState.value.selectedZone)
        }

    @Test
    fun `load fetches clusters for the initial whole-zone viewport`() =
        runTest {
            val zone = aWatchZone()
            val watchZoneRepository = FakeWatchZoneRepository(mutableListOf(zone))
            val planningRepository = FakePlanningApplicationRepository()
            val sut = makeSut(watchZoneRepository = watchZoneRepository, planningRepository = planningRepository)

            sut.load()

            val call = planningRepository.fetchClustersCalls.single()
            assertEquals(zone.id, call.zoneId)
            val (expectedViewport, expectedZoom) = MapViewport.initial(zone.centre, zone.radiusMetres)
            assertEquals(expectedViewport, call.viewport)
            assertEquals(expectedZoom, call.zoom)
        }

    @Test
    fun `load marks hasLoaded even when there is no zone at all`() =
        runTest {
            val sut = makeSut()

            sut.load()

            assertTrue(sut.uiState.value.hasLoaded)
            assertNull(sut.uiState.value.selectedZone)
        }

    @Test
    fun `selectZone persists the choice, resets the status filter, and refetches immediately`() =
        runTest {
            val home = aWatchZone(id = WatchZoneId("wz-home"))
            val office = aWatchZone(id = WatchZoneId("wz-office"))
            val watchZoneRepository = FakeWatchZoneRepository(mutableListOf(home, office))
            val planningRepository = FakePlanningApplicationRepository()
            val mapPreferencesStore = FakeMapPreferencesStore()
            val sut =
                makeSut(
                    watchZoneRepository = watchZoneRepository,
                    planningRepository = planningRepository,
                    mapPreferencesStore = mapPreferencesStore,
                )
            sut.load()
            sut.applyStatusFilter(ApplicationStatus.Permitted)

            sut.selectZone(office)

            assertEquals(office, sut.uiState.value.selectedZone)
            assertNull(sut.uiState.value.selectedStatusFilter)
            assertEquals(WatchZoneId("wz-office"), mapPreferencesStore.storedZoneId)
            assertEquals(office.id, planningRepository.fetchClustersCalls.last().zoneId)
        }

    @Test
    fun `selectZone sets a pending camera target seeded from the zone's whole-zone viewport`() =
        runTest {
            val zone = aWatchZone()
            val sut = makeSut()

            sut.selectZone(zone)

            assertNotNull(sut.uiState.value.pendingCameraTarget)
        }

    @Test
    fun `consumeCameraTarget clears the one-shot camera effect`() =
        runTest {
            val sut = makeSut()
            sut.selectZone(aWatchZone())

            sut.consumeCameraTarget()

            assertNull(sut.uiState.value.pendingCameraTarget)
        }

    @Test
    fun `applyStatusFilter sends the raw status and refetches the last viewport`() =
        runTest {
            val zone = aWatchZone()
            val watchZoneRepository = FakeWatchZoneRepository(mutableListOf(zone))
            val planningRepository = FakePlanningApplicationRepository()
            val sut = makeSut(watchZoneRepository = watchZoneRepository, planningRepository = planningRepository)
            sut.load()

            sut.applyStatusFilter(ApplicationStatus.Permitted)

            val call = planningRepository.fetchClustersCalls.last()
            assertEquals(ApplicationStatus.Permitted, call.status)
        }

    @Test
    fun `applyStatusFilter with All (null) clears the filter and sends no status`() =
        runTest {
            val zone = aWatchZone()
            val watchZoneRepository = FakeWatchZoneRepository(mutableListOf(zone))
            val planningRepository = FakePlanningApplicationRepository()
            val sut = makeSut(watchZoneRepository = watchZoneRepository, planningRepository = planningRepository)
            sut.load()
            sut.applyStatusFilter(ApplicationStatus.Permitted)

            sut.applyStatusFilter(null)

            val call = planningRepository.fetchClustersCalls.last()
            assertNull(call.status)
            assertNull(sut.uiState.value.selectedStatusFilter)
        }

    @Test
    fun `a refetch failure keeps the last good clusters and does not surface an error`() =
        runTest {
            val zone = aWatchZone()
            val watchZoneRepository = FakeWatchZoneRepository(mutableListOf(zone))
            val planningRepository =
                FakePlanningApplicationRepository().apply {
                    fetchClustersResult = listOf(aSingleMemberMapCluster())
                }
            val sut = makeSut(watchZoneRepository = watchZoneRepository, planningRepository = planningRepository)
            sut.load()
            assertEquals(1, sut.uiState.value.clusters.size)

            planningRepository.fetchClustersFailWith = DomainError.NetworkUnavailable
            sut.applyStatusFilter(ApplicationStatus.Permitted)

            assertEquals(1, sut.uiState.value.clusters.size)
            assertNull(sut.uiState.value.error)
        }

    @Test
    fun `a load failure with nothing loaded yet surfaces the error`() =
        runTest {
            val zone = aWatchZone()
            val watchZoneRepository = FakeWatchZoneRepository(mutableListOf(zone))
            val planningRepository =
                FakePlanningApplicationRepository().apply {
                    fetchClustersFailWith = DomainError.NetworkUnavailable
                }
            val sut = makeSut(watchZoneRepository = watchZoneRepository, planningRepository = planningRepository)

            sut.load()

            assertEquals(DomainError.NetworkUnavailable, sut.uiState.value.error)
        }

    @Test
    fun `selectFromStack closes the disambiguation list and opens the chosen application's summary`() =
        runTest {
            val application = aPlanningApplication()
            val sut = makeSut()

            sut.selectFromStack(application)

            assertEquals(application, sut.uiState.value.selectedApplication)
            assertNull(sut.uiState.value.stackedApplications)
        }

    @Test
    fun `clearSelection dismisses the summary sheet`() =
        runTest {
            val application = aPlanningApplication()
            val sut = makeSut()
            sut.selectFromStack(application)

            sut.clearSelection()

            assertNull(sut.uiState.value.selectedApplication)
        }

    @Test
    fun `toggleSaveSelectedApplication optimistically flips isSelectedApplicationSaved and persists it`() =
        runTest {
            val application = aPlanningApplication()
            val savedApplicationRepository = FakeSavedApplicationRepository()
            val sut = makeSut(savedApplicationRepository = savedApplicationRepository)
            sut.selectFromStack(application)

            sut.toggleSaveSelectedApplication()

            assertTrue(sut.uiState.value.isSelectedApplicationSaved)
            assertEquals(listOf(application.id), savedApplicationRepository.saveCalls)
        }

    @Test
    fun `toggleSaveSelectedApplication reverts on failure`() =
        runTest {
            val application = aPlanningApplication()
            val savedApplicationRepository =
                FakeSavedApplicationRepository().apply { saveFailWith = DomainError.NetworkUnavailable }
            val sut = makeSut(savedApplicationRepository = savedApplicationRepository)
            sut.selectFromStack(application)

            sut.toggleSaveSelectedApplication()

            assertTrue(!sut.uiState.value.isSelectedApplicationSaved)
        }

    @Test
    fun `selectFromStack loads the saved state for the chosen application`() =
        runTest {
            val application = aPlanningApplication()
            val savedApplicationRepository =
                FakeSavedApplicationRepository(
                    stored = mutableListOf(aSavedApplication(applicationUid = application.id)),
                )
            val sut = makeSut(savedApplicationRepository = savedApplicationRepository)

            sut.selectFromStack(application)

            assertTrue(sut.uiState.value.isSelectedApplicationSaved)
        }
}
