package uk.towncrierapp.presentation.features.map

import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.applications.FakePlanningApplicationRepository
import uk.towncrierapp.domain.applications.FakeSavedApplicationRepository
import uk.towncrierapp.domain.applications.aPlanningApplication
import uk.towncrierapp.domain.applications.aPlanningApplicationId
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.map.FakeMapPreferencesStore
import uk.towncrierapp.domain.map.MapViewport
import uk.towncrierapp.domain.map.aSingleMemberMapCluster
import uk.towncrierapp.domain.map.aSplittableMapCluster
import uk.towncrierapp.domain.map.aStackedMapCluster
import uk.towncrierapp.domain.map.zoomedInto
import uk.towncrierapp.domain.watchzones.FakeWatchZoneRepository
import uk.towncrierapp.domain.watchzones.aWatchZone
import uk.towncrierapp.presentation.MainDispatcherExtension
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * [MapViewModel.onClusterTapped] routing (GH#776, GH#722): a single-member
 * cell point-reads and opens the summary sheet; a splittable bubble computes
 * a client-side zoom-in target with no network call; a stacked cell
 * point-reads every member concurrently, all-or-nothing. Port of iOS
 * `MapViewModelDetailTests`/`MapViewModelStackTests`.
 */
@ExtendWith(MainDispatcherExtension::class)
class MapViewModelTapRoutingTest {
    private fun makeSut(
        planningRepository: FakePlanningApplicationRepository = FakePlanningApplicationRepository(),
        watchZoneRepository: FakeWatchZoneRepository = FakeWatchZoneRepository(mutableListOf(aWatchZone())),
    ) = MapViewModel(
        planningRepository,
        watchZoneRepository,
        FakeSavedApplicationRepository(),
        FakeMapPreferencesStore(),
    )

    @Test
    fun `tapping a single-member cluster point-reads and opens its summary`() =
        runTest {
            val application = aPlanningApplication(id = aPlanningApplicationId(authority = "42", name = "24/0001"))
            val planningRepository = FakePlanningApplicationRepository().apply { detailResult = application }
            val sut = makeSut(planningRepository = planningRepository)
            val cluster = aSingleMemberMapCluster(member = application.id)

            sut.onClusterTapped(cluster)

            assertEquals(listOf("42" to "24/0001"), planningRepository.detailCalls)
            assertEquals(application, sut.uiState.value.selectedApplication)
        }

    @Test
    fun `a single-member point-read failure leaves the map untouched`() =
        runTest {
            val planningRepository =
                FakePlanningApplicationRepository().apply { detailFailWith = DomainError.NetworkUnavailable }
            val sut = makeSut(planningRepository = planningRepository)

            sut.onClusterTapped(aSingleMemberMapCluster())

            assertNull(sut.uiState.value.selectedApplication)
            assertNull(sut.uiState.value.error)
        }

    @Test
    fun `tapping a splittable bubble computes a halved, floored zoom-in target with no network call`() =
        runTest {
            val zone = aWatchZone()
            val planningRepository = FakePlanningApplicationRepository()
            val sut =
                makeSut(
                    planningRepository = planningRepository,
                    watchZoneRepository = FakeWatchZoneRepository(mutableListOf(zone)),
                )
            sut.load()
            val viewportBeforeTap = MapViewport.initial(zone.centre, zone.radiusMetres).first
            val cluster = aSplittableMapCluster(coordinate = zone.centre, count = 5)

            sut.onClusterTapped(cluster)

            assertEquals(viewportBeforeTap.zoomedInto(zone.centre), sut.uiState.value.pendingCameraTarget)
            assertTrue(planningRepository.detailCalls.isEmpty())
        }

    @Test
    fun `tapping a stacked cluster point-reads every member concurrently and opens the disambiguation list`() =
        runTest {
            val first = aPlanningApplication(id = aPlanningApplicationId(authority = "42", name = "24/0001"))
            val second = aPlanningApplication(id = aPlanningApplicationId(authority = "42", name = "24/0002"))
            val planningRepository =
                FakePlanningApplicationRepository().apply {
                    detailResults[first.id.value] = first
                    detailResults[second.id.value] = second
                }
            val sut = makeSut(planningRepository = planningRepository)
            val cluster = aStackedMapCluster(members = listOf(first.id, second.id))

            sut.onClusterTapped(cluster)

            assertEquals(listOf(first, second), sut.uiState.value.stackedApplications)
            assertNull(sut.uiState.value.selectedApplication)
        }

    @Test
    fun `a stacked cluster is all-or-nothing — one member failing shows no list and leaves the map untouched`() =
        runTest {
            val ok = aPlanningApplicationId(authority = "42", name = "24/0001")
            val failing = aPlanningApplicationId(authority = "42", name = "24/0002")
            val planningRepository =
                FakePlanningApplicationRepository().apply {
                    detailResults[ok.value] = aPlanningApplication(id = ok)
                    detailFailFor[failing.value] = DomainError.NetworkUnavailable
                }
            val sut = makeSut(planningRepository = planningRepository)
            val cluster = aStackedMapCluster(members = listOf(ok, failing))

            sut.onClusterTapped(cluster)

            assertNull(sut.uiState.value.stackedApplications)
        }
}
