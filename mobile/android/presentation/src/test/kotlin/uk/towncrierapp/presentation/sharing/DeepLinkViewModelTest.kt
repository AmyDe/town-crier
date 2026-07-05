package uk.towncrierapp.presentation.sharing

import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.applications.FakePlanningApplicationRepository
import uk.towncrierapp.domain.applications.PlanningApplicationId
import uk.towncrierapp.domain.applications.aPlanningApplication
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.presentation.MainDispatcherExtension
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * Resolves a parsed [DeepLink] into a navigable [DeepLinkResolution] — the
 * legacy `/applications/{uid}` shape via the authed by-id read, the public
 * `/a/{slug}/{ref}` share shape via the anonymous by-slug read, and the bare
 * `/applications` shape with no fetch at all. Only the by-id path fires the
 * review-prompt `openedAlert` signal — arriving via a share link deliberately
 * does not (GH#782, iOS parity).
 */
@ExtendWith(MainDispatcherExtension::class)
class DeepLinkViewModelTest {
    @Test
    fun `resolving ApplicationsList surfaces ShowApplicationsList with no fetch`() =
        runTest {
            val repository = FakePlanningApplicationRepository()
            val viewModel = DeepLinkViewModel(repository)

            viewModel.resolve(DeepLink.ApplicationsList)

            assertEquals(DeepLinkResolution.ShowApplicationsList, viewModel.uiState.value.resolution)
            assertTrue(repository.detailCalls.isEmpty())
            assertTrue(repository.detailBySlugCalls.isEmpty())
        }

    @Test
    fun `resolving ApplicationDetail fetches by id and surfaces ShowApplication`() =
        runTest {
            val application = aPlanningApplication()
            val repository = FakePlanningApplicationRepository().apply { detailResult = application }
            val viewModel = DeepLinkViewModel(repository)

            viewModel.resolve(DeepLink.ApplicationDetail(PlanningApplicationId(authority = "42", name = "24/0001")))

            val resolution = assertIs<DeepLinkResolution.ShowApplication>(viewModel.uiState.value.resolution)
            assertEquals(application, resolution.application)
            assertEquals(listOf("42" to "24/0001"), repository.detailCalls)
        }

    @Test
    fun `resolving ApplicationDetail fires the openedAlert callback`() =
        runTest {
            val repository = FakePlanningApplicationRepository()
            var openedAlertCalls = 0
            val viewModel = DeepLinkViewModel(repository, onOpenedAlert = { openedAlertCalls++ })

            viewModel.resolve(DeepLink.ApplicationDetail(PlanningApplicationId(authority = "42", name = "24/0001")))

            assertEquals(1, openedAlertCalls)
        }

    @Test
    fun `resolving ShareApplication fetches by slug and surfaces ShowApplication`() =
        runTest {
            val application = aPlanningApplication()
            val repository = FakePlanningApplicationRepository().apply { detailBySlugResult = application }
            val viewModel = DeepLinkViewModel(repository)

            viewModel.resolve(DeepLink.ShareApplication(authoritySlug = "camden", ref = "24/0001"))

            val resolution = assertIs<DeepLinkResolution.ShowApplication>(viewModel.uiState.value.resolution)
            assertEquals(application, resolution.application)
            assertEquals(listOf("camden" to "24/0001"), repository.detailBySlugCalls)
        }

    @Test
    fun `resolving ShareApplication does NOT fire the openedAlert callback`() =
        runTest {
            val repository = FakePlanningApplicationRepository()
            var openedAlertCalls = 0
            val viewModel = DeepLinkViewModel(repository, onOpenedAlert = { openedAlertCalls++ })

            viewModel.resolve(DeepLink.ShareApplication(authoritySlug = "camden", ref = "24/0001"))

            assertEquals(0, openedAlertCalls)
        }

    @Test
    fun `a failed by-slug resolution surfaces the error, not a resolution`() =
        runTest {
            val repository = FakePlanningApplicationRepository().apply { detailBySlugFailWith = DomainError.NotFound }
            val viewModel = DeepLinkViewModel(repository)

            viewModel.resolve(DeepLink.ShareApplication(authoritySlug = "unknown-slug", ref = "24/0001"))

            assertNull(viewModel.uiState.value.resolution)
            assertEquals(DomainError.NotFound, viewModel.uiState.value.error)
        }

    @Test
    fun `consumeResolution clears the resolution once dispatched`() =
        runTest {
            val repository = FakePlanningApplicationRepository()
            val viewModel = DeepLinkViewModel(repository)
            viewModel.resolve(DeepLink.ApplicationsList)

            viewModel.consumeResolution()

            assertNull(viewModel.uiState.value.resolution)
        }
}
