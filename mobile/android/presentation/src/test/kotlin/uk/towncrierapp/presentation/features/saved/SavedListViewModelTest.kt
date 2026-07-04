package uk.towncrierapp.presentation.features.saved

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.FakeSavedApplicationRepository
import uk.towncrierapp.domain.applications.aPlanningApplication
import uk.towncrierapp.domain.applications.aPlanningApplicationId
import uk.towncrierapp.domain.applications.aSavedApplication
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.presentation.MainDispatcherExtension
import java.time.OffsetDateTime
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** Port of iOS `SavedApplicationListViewModelTests` (GH#775). */
@ExtendWith(MainDispatcherExtension::class)
class SavedListViewModelTest {
    @Test
    fun `load fetches the flat saved list`() {
        val repository = FakeSavedApplicationRepository(mutableListOf(aSavedApplication()))
        val viewModel = SavedListViewModel(repository)

        viewModel.load()

        assertEquals(1, viewModel.uiState.value.savedApplications.size)
    }

    @Test
    fun `load does not refetch once it has already succeeded — tc-hlbx`() {
        val repository = FakeSavedApplicationRepository(mutableListOf(aSavedApplication()))
        val viewModel = SavedListViewModel(repository)
        viewModel.load()

        viewModel.load()

        assertEquals(1, repository.savedApplicationsCallCount)
    }

    @Test
    fun `load retries after a previous attempt failed — tc-hlbx`() {
        val repository =
            FakeSavedApplicationRepository().apply {
                savedApplicationsFailWith =
                    DomainError.NetworkUnavailable
            }
        val viewModel = SavedListViewModel(repository)
        viewModel.load()
        assertEquals(1, repository.savedApplicationsCallCount)

        repository.savedApplicationsFailWith = null
        viewModel.load()

        assertEquals(2, repository.savedApplicationsCallCount)
        assertNull(viewModel.uiState.value.error)
    }

    @Test
    fun `a legacy row with a null application payload is dropped from the displayed list`() {
        val legacy = aSavedApplication(applicationUid = aPlanningApplicationId(name = "24/9999"), application = null)
        val real = aSavedApplication(applicationUid = aPlanningApplicationId(name = "24/0001"))
        val repository = FakeSavedApplicationRepository(mutableListOf(legacy, real))
        val viewModel = SavedListViewModel(repository)

        viewModel.load()

        assertEquals(2, viewModel.uiState.value.savedApplications.size)
        assertEquals(listOf(real), viewModel.uiState.value.displayed)
    }

    @Test
    fun `selecting a status filter narrows the displayed list client-side`() {
        val permitted =
            aSavedApplication(
                applicationUid = aPlanningApplicationId(name = "24/0001"),
                application =
                    aPlanningApplication(
                        id = aPlanningApplicationId(name = "24/0001"),
                        status = ApplicationStatus.Permitted,
                    ),
            )
        val rejected =
            aSavedApplication(
                applicationUid = aPlanningApplicationId(name = "24/0002"),
                application =
                    aPlanningApplication(
                        id = aPlanningApplicationId(name = "24/0002"),
                        status = ApplicationStatus.Rejected,
                    ),
            )
        val repository = FakeSavedApplicationRepository(mutableListOf(permitted, rejected))
        val viewModel = SavedListViewModel(repository)
        viewModel.load()

        viewModel.selectFilter(ApplicationStatus.Permitted)

        assertEquals(listOf(permitted), viewModel.uiState.value.displayed)
    }

    @Test
    fun `a null filter shows every (non-legacy) saved application`() {
        val a = aSavedApplication(applicationUid = aPlanningApplicationId(name = "24/0001"))
        val b = aSavedApplication(applicationUid = aPlanningApplicationId(name = "24/0002"))
        val repository = FakeSavedApplicationRepository(mutableListOf(a, b))
        val viewModel = SavedListViewModel(repository)
        viewModel.load()
        viewModel.selectFilter(ApplicationStatus.Permitted)

        viewModel.selectFilter(null)

        assertEquals(2, viewModel.uiState.value.displayed.size)
    }

    @Test
    fun `displayed is sorted by savedAt descending regardless of wire order`() {
        val older =
            aSavedApplication(
                applicationUid = aPlanningApplicationId(name = "24/0001"),
                savedAt = OffsetDateTime.parse("2026-01-01T00:00:00Z"),
            )
        val newer =
            aSavedApplication(
                applicationUid = aPlanningApplicationId(name = "24/0002"),
                savedAt = OffsetDateTime.parse("2026-02-01T00:00:00Z"),
            )
        val repository = FakeSavedApplicationRepository(mutableListOf(older, newer))
        val viewModel = SavedListViewModel(repository)

        viewModel.load()

        assertEquals(listOf(newer, older), viewModel.uiState.value.displayed)
        assertTrue(
            viewModel.uiState.value.displayed
                .first()
                .savedAt
                .isAfter(
                    viewModel.uiState.value.displayed
                        .last()
                        .savedAt,
                ),
        )
    }
}
