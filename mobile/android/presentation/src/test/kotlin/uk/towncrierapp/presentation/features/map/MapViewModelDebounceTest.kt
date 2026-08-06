package uk.towncrierapp.presentation.features.map

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import uk.towncrierapp.domain.applications.FakePlanningApplicationRepository
import uk.towncrierapp.domain.applications.FakeSavedApplicationRepository
import uk.towncrierapp.domain.map.FakeMapPreferencesStore
import uk.towncrierapp.domain.map.MapViewport
import uk.towncrierapp.domain.map.aMapViewport
import uk.towncrierapp.domain.watchzones.FakeWatchZoneRepository
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.domain.watchzones.aWatchZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * Camera-idle debounce (GH#776): a pan/zoom flurry coalesces into a single
 * fetch ~250ms after it settles. Needs real virtual-time control over
 * `delay()` (not the shared [uk.towncrierapp.presentation.MainDispatcherExtension]'s
 * eager [kotlinx.coroutines.test.UnconfinedTestDispatcher] — testing.md's
 * documented caveat), so this suite wires its own [StandardTestDispatcher]
 * shared with [runTest]'s scheduler.
 */
class MapViewModelDebounceTest {
    private val testDispatcher = StandardTestDispatcher()

    @BeforeEach
    fun setUp() {
        Dispatchers.setMain(testDispatcher)
    }

    @AfterEach
    fun tearDown() {
        Dispatchers.resetMain()
    }

    private fun makeSut(planningRepository: FakePlanningApplicationRepository): MapViewModel {
        val zone = aWatchZone()
        val watchZoneRepository = FakeWatchZoneRepository(mutableListOf(zone))
        return MapViewModel(
            planningRepository,
            watchZoneRepository,
            FakeSavedApplicationRepository(),
            FakeMapPreferencesStore(),
        )
    }

    @Test
    fun `a burst of camera-idle events coalesces to exactly one fetch after the debounce settles`() =
        runTest(testDispatcher) {
            val planningRepository = FakePlanningApplicationRepository()
            val sut = makeSut(planningRepository)
            sut.load()
            runCurrent()
            val callsAfterLoad = planningRepository.fetchClustersCalls.size

            val first = aMapViewport(west = 0.0, south = 0.0, east = 0.1, north = 0.1)
            val second = aMapViewport(west = 1.0, south = 1.0, east = 1.1, north = 1.1)
            val third = aMapViewport(west = 2.0, south = 2.0, east = 2.1, north = 2.1)
            sut.onCameraIdle(first, rawZoom = 10.0)
            advanceTimeBy(100)
            sut.onCameraIdle(second, rawZoom = 10.0)
            advanceTimeBy(100)
            sut.onCameraIdle(third, rawZoom = 10.0)
            advanceTimeBy(300)
            runCurrent()

            val newCalls = planningRepository.fetchClustersCalls.drop(callsAfterLoad)
            assertEquals(1, newCalls.size)
            assertEquals(third, newCalls.single().viewport)
        }

    @Test
    fun `a fetch is not issued before the debounce window elapses`() =
        runTest(testDispatcher) {
            val planningRepository = FakePlanningApplicationRepository()
            val sut = makeSut(planningRepository)
            sut.load()
            runCurrent()
            val callsAfterLoad = planningRepository.fetchClustersCalls.size

            sut.onCameraIdle(aMapViewport(), rawZoom = 10.0)
            advanceTimeBy(100)
            runCurrent()

            assertEquals(callsAfterLoad, planningRepository.fetchClustersCalls.size)
        }

    @Test
    fun `switching zones cancels a pending debounced pan-fetch so it never lands with the stale zone's viewport`() =
        runTest(testDispatcher) {
            val zoneA = aWatchZone(id = WatchZoneId("wz-a"))
            val zoneB = aWatchZone(id = WatchZoneId("wz-b"))
            val watchZoneRepository = FakeWatchZoneRepository(mutableListOf(zoneA, zoneB))
            val planningRepository = FakePlanningApplicationRepository()
            val sut =
                MapViewModel(
                    planningRepository,
                    watchZoneRepository,
                    FakeSavedApplicationRepository(),
                    FakeMapPreferencesStore(),
                )
            sut.load()
            runCurrent()
            val zoneAPanViewport = aMapViewport(west = 5.0, south = 5.0, east = 5.1, north = 5.1)
            sut.onCameraIdle(zoneAPanViewport, rawZoom = 10.0)

            sut.selectZone(zoneB)
            runCurrent()
            advanceTimeBy(300)
            runCurrent()

            val zoneBInitialViewport = MapViewport.initial(zoneB.centre, zoneB.radiusMetres).first
            assertTrue(planningRepository.fetchClustersCalls.none { it.viewport == zoneAPanViewport })
            val lastCall = planningRepository.fetchClustersCalls.last()
            assertEquals(zoneB.id, lastCall.zoneId)
            assertEquals(zoneBInitialViewport, lastCall.viewport)
        }

    @Test
    fun `a fractional or out-of-range camera zoom is rounded and clamped into 0 to 20 before fetching`() =
        runTest(testDispatcher) {
            val planningRepository = FakePlanningApplicationRepository()
            val sut = makeSut(planningRepository)
            sut.load()
            runCurrent()

            sut.onCameraIdle(aMapViewport(), rawZoom = 24.7)
            advanceTimeBy(300)
            runCurrent()

            assertEquals(20, planningRepository.fetchClustersCalls.last().zoom)
        }
}
