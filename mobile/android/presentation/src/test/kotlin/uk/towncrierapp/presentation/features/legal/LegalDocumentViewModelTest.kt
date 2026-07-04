package uk.towncrierapp.presentation.features.legal

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.legal.FakeLegalDocumentRepository
import uk.towncrierapp.domain.legal.LegalDocumentSection
import uk.towncrierapp.domain.legal.LegalDocumentType
import uk.towncrierapp.domain.legal.aLegalDocument
import uk.towncrierapp.presentation.MainDispatcherExtension
import java.time.LocalDate
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/** Port of iOS `LegalDocumentViewModelTests`, adapted to the async bundled-asset load (GH#778). */
@ExtendWith(MainDispatcherExtension::class)
class LegalDocumentViewModelTest {
    @Test
    fun `before load, state is loading with no content`() {
        val viewModel = LegalDocumentViewModel(FakeLegalDocumentRepository(), LegalDocumentType.PRIVACY_POLICY)

        assertTrue(viewModel.uiState.value.isLoading)
        assertTrue(viewModel.uiState.value.sections.isEmpty())
    }

    @Test
    fun `load populates the title and formats lastUpdated as d MMMM yyyy en-GB`() {
        val repository =
            FakeLegalDocumentRepository(
                documentResult =
                    Result.success(
                        aLegalDocument(title = "Privacy Policy", lastUpdated = LocalDate.of(2026, 7, 1)),
                    ),
            )
        val viewModel = LegalDocumentViewModel(repository, LegalDocumentType.PRIVACY_POLICY)

        viewModel.load()

        assertEquals("Privacy Policy", viewModel.uiState.value.title)
        assertEquals("1 July 2026", viewModel.uiState.value.formattedLastUpdated)
        assertEquals(false, viewModel.uiState.value.isLoading)
    }

    @Test
    fun `load populates every section heading and body`() {
        val repository =
            FakeLegalDocumentRepository(
                documentResult =
                    Result.success(
                        aLegalDocument(
                            sections =
                                listOf(
                                    LegalDocumentSection("Who We Are", "Town Crier."),
                                    LegalDocumentSection("What We Collect", "A unique user ID."),
                                ),
                        ),
                    ),
            )
        val viewModel = LegalDocumentViewModel(repository, LegalDocumentType.TERMS_OF_SERVICE)

        viewModel.load()

        assertEquals(2, viewModel.uiState.value.sections.size)
        assertEquals("Who We Are", viewModel.uiState.value.sections[0].heading)
        assertEquals("Town Crier.", viewModel.uiState.value.sections[0].body)
        assertEquals("What We Collect", viewModel.uiState.value.sections[1].heading)
    }

    @Test
    fun `load requests the document for the constructor-supplied type`() {
        val repository = FakeLegalDocumentRepository()
        val viewModel = LegalDocumentViewModel(repository, LegalDocumentType.TERMS_OF_SERVICE)

        viewModel.load()

        assertEquals(listOf(LegalDocumentType.TERMS_OF_SERVICE), repository.documentCalls)
    }
}
