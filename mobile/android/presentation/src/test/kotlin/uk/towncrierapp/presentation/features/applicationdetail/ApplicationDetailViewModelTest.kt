package uk.towncrierapp.presentation.features.applicationdetail

import kotlinx.coroutines.CompletableDeferred
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.applications.FakePlanningApplicationRepository
import uk.towncrierapp.domain.applications.FakeSavedApplicationRepository
import uk.towncrierapp.domain.applications.aLocalAuthority
import uk.towncrierapp.domain.applications.aPlanningApplication
import uk.towncrierapp.domain.applications.aPlanningApplicationId
import uk.towncrierapp.domain.applications.aSavedApplication
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.presentation.MainDispatcherExtension
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** Port of iOS `ApplicationDetailViewModelTests` (GH#775). */
@ExtendWith(MainDispatcherExtension::class)
class ApplicationDetailViewModelTest {
    @Test
    fun `the passed-in application renders instantly, before any refresh completes`() {
        val initial = aPlanningApplication(description = "Initial description")
        val viewModel = ApplicationDetailViewModel(FakePlanningApplicationRepository(), FakeSavedApplicationRepository(), initial)

        assertEquals(initial, viewModel.uiState.value.application)
        assertFalse(viewModel.uiState.value.isRefreshing)
    }

    @Test
    fun `refresh replaces the rendered application with the fresh by-id fetch`() {
        val initial = aPlanningApplication(description = "Initial description")
        val fresh = aPlanningApplication(description = "Fresh description")
        val repository = FakePlanningApplicationRepository().apply { detailResult = fresh }
        val viewModel = ApplicationDetailViewModel(repository, FakeSavedApplicationRepository(), initial)

        viewModel.refresh()

        assertEquals(fresh, viewModel.uiState.value.application)
        assertFalse(viewModel.uiState.value.isRefreshing)
        assertEquals(initial.id.authority to initial.id.name, repository.detailCalls.single())
    }

    @Test
    fun `a second rapid refresh while one is already in flight is ignored (re-entrancy guard)`() {
        val repository = FakePlanningApplicationRepository()
        val gate = CompletableDeferred<Unit>()
        repository.beforeDetail = { gate.await() }
        val viewModel = ApplicationDetailViewModel(repository, FakeSavedApplicationRepository(), aPlanningApplication())

        viewModel.refresh()
        viewModel.refresh()

        assertEquals(1, repository.detailCalls.size)
        assertTrue(viewModel.uiState.value.isRefreshing)

        gate.complete(Unit)

        assertFalse(viewModel.uiState.value.isRefreshing)
        assertEquals(1, repository.detailCalls.size)
    }

    @Test
    fun `a refresh failure surfaces an error but keeps the stale application rendered`() {
        val initial = aPlanningApplication()
        val repository = FakePlanningApplicationRepository().apply { detailFailWith = DomainError.NetworkUnavailable }
        val viewModel = ApplicationDetailViewModel(repository, FakeSavedApplicationRepository(), initial)

        viewModel.refresh()

        assertEquals(initial, viewModel.uiState.value.application)
        assertEquals(DomainError.NetworkUnavailable, viewModel.uiState.value.error)
    }

    @Test
    fun `share is disabled until the by-id refresh supplies an authoritySlug`() {
        val initial = aPlanningApplication(authority = aLocalAuthority(slug = null))
        val repository =
            FakePlanningApplicationRepository().apply { detailResult = aPlanningApplication(authority = aLocalAuthority(slug = "camden")) }
        val viewModel = ApplicationDetailViewModel(repository, FakeSavedApplicationRepository(), initial)

        assertFalse(viewModel.uiState.value.canShare)

        viewModel.refresh()

        assertTrue(viewModel.uiState.value.canShare)
        assertEquals("camden", viewModel.uiState.value.authoritySlug)
    }

    @Test
    fun `checkSavedState compares by the reconstructed PlanningApplicationId, not a raw uid string`() {
        val id = aPlanningApplicationId(authority = "42", name = "24/0001")
        val application = aPlanningApplication(id = id)
        val savedRepository = FakeSavedApplicationRepository(mutableListOf(aSavedApplication(applicationUid = id)))
        val viewModel = ApplicationDetailViewModel(FakePlanningApplicationRepository(), savedRepository, application)

        viewModel.checkSavedState()

        assertTrue(viewModel.uiState.value.isSaved)
    }

    @Test
    fun `checkSavedState is false when no saved row's reconstructed id matches`() {
        val application = aPlanningApplication(id = aPlanningApplicationId(name = "24/0001"))
        val savedRepository =
            FakeSavedApplicationRepository(mutableListOf(aSavedApplication(applicationUid = aPlanningApplicationId(name = "24/9999"))))
        val viewModel = ApplicationDetailViewModel(FakePlanningApplicationRepository(), savedRepository, application)

        viewModel.checkSavedState()

        assertFalse(viewModel.uiState.value.isSaved)
    }

    @Test
    fun `toggleSave saves when not currently saved`() {
        val application = aPlanningApplication()
        val savedRepository = FakeSavedApplicationRepository()
        val viewModel = ApplicationDetailViewModel(FakePlanningApplicationRepository(), savedRepository, application)

        viewModel.toggleSave()

        assertTrue(viewModel.uiState.value.isSaved)
        assertEquals(listOf(application.id), savedRepository.saveCalls)
        assertTrue(savedRepository.unsaveCalls.isEmpty())
    }

    @Test
    fun `toggleSave unsaves when currently saved`() {
        val id = aPlanningApplicationId()
        val application = aPlanningApplication(id = id)
        val savedRepository = FakeSavedApplicationRepository(mutableListOf(aSavedApplication(applicationUid = id)))
        val viewModel = ApplicationDetailViewModel(FakePlanningApplicationRepository(), savedRepository, application)
        viewModel.checkSavedState()

        viewModel.toggleSave()

        assertFalse(viewModel.uiState.value.isSaved)
        assertEquals(listOf(id), savedRepository.unsaveCalls)
    }

    @Test
    fun `toggleSave reverts the optimistic state and surfaces an error on failure`() {
        val application = aPlanningApplication()
        val savedRepository = FakeSavedApplicationRepository().apply { saveFailWith = DomainError.NetworkUnavailable }
        val viewModel = ApplicationDetailViewModel(FakePlanningApplicationRepository(), savedRepository, application)

        viewModel.toggleSave()

        assertFalse(viewModel.uiState.value.isSaved)
        assertEquals(DomainError.NetworkUnavailable, viewModel.uiState.value.error)
    }
}
