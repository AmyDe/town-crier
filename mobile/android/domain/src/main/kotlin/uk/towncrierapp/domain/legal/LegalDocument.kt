package uk.towncrierapp.domain.legal

import java.time.LocalDate

/** Which bundled legal document to load. */
public enum class LegalDocumentType {
    PRIVACY_POLICY,
    TERMS_OF_SERVICE,
}

/** One section of a legal document: a heading and its body text. */
public data class LegalDocumentSection(
    val heading: String,
    val body: String,
)

/**
 * A bundled legal document (privacy policy or terms of service). Content is
 * loaded from JSON bundled with the app — the same JSON files embedded on
 * the API side (canonical source); `scripts/check-legal-drift.sh` enforces
 * byte-equality. Port of iOS's `LegalDocumentViewModel`'s decoded shape.
 */
public data class LegalDocument(
    val title: String,
    val lastUpdated: LocalDate,
    val sections: List<LegalDocumentSection>,
)
