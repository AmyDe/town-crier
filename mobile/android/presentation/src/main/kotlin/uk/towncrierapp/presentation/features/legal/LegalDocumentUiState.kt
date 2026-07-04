package uk.towncrierapp.presentation.features.legal

/** One section of a legal document, ready to render. */
public data class LegalDocumentSectionUi(
    val heading: String,
    val body: String,
)

/** State for [LegalDocumentScreen]/[LegalDocumentViewModel]. */
public data class LegalDocumentUiState(
    val isLoading: Boolean = true,
    val title: String = "",
    val formattedLastUpdated: String = "",
    val sections: List<LegalDocumentSectionUi> = emptyList(),
)
