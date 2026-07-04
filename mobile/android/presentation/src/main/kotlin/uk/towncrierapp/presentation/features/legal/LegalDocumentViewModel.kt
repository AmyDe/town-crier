package uk.towncrierapp.presentation.features.legal

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import uk.towncrierapp.domain.legal.LegalDocumentRepository
import uk.towncrierapp.domain.legal.LegalDocumentType
import java.time.format.DateTimeFormatter
import java.util.Locale

private val LAST_UPDATED_FORMATTER: DateTimeFormatter = DateTimeFormatter.ofPattern("d MMMM yyyy", Locale.UK)

/**
 * Drives a single bundled legal document screen (privacy policy or terms of
 * service): title, `d MMMM yyyy` en-GB formatted date, and section list.
 * Port of iOS `LegalDocumentViewModel`, adapted to the async bundled-asset
 * load `:data`'s `LegalDocumentLoader` performs.
 */
public class LegalDocumentViewModel(
    private val repository: LegalDocumentRepository,
    private val documentType: LegalDocumentType,
) : ViewModel() {
    private val _uiState = MutableStateFlow(LegalDocumentUiState())
    public val uiState: StateFlow<LegalDocumentUiState> = _uiState.asStateFlow()

    public fun load() {
        viewModelScope.launch {
            val document = repository.document(documentType)
            _uiState.update {
                it.copy(
                    isLoading = false,
                    title = document.title,
                    formattedLastUpdated = document.lastUpdated.format(LAST_UPDATED_FORMATTER),
                    sections =
                        document.sections.map { section ->
                            LegalDocumentSectionUi(section.heading, section.body)
                        },
                )
            }
        }
    }
}
