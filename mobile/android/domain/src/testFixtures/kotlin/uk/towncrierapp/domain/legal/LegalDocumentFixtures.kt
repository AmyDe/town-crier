package uk.towncrierapp.domain.legal

import java.time.LocalDate

/** Fixture factory for [LegalDocument]. */
public fun aLegalDocument(
    title: String = "Privacy Policy",
    lastUpdated: LocalDate = LocalDate.of(2026, 7, 1),
    sections: List<LegalDocumentSection> = listOf(LegalDocumentSection(heading = "Who We Are", body = "Town Crier.")),
): LegalDocument = LegalDocument(title = title, lastUpdated = lastUpdated, sections = sections)

/** Hand-written fake for [LegalDocumentRepository]. */
public class FakeLegalDocumentRepository(
    public var documentResult: Result<LegalDocument> = Result.success(aLegalDocument()),
) : LegalDocumentRepository {
    public val documentCalls: MutableList<LegalDocumentType> = mutableListOf()

    override suspend fun document(type: LegalDocumentType): LegalDocument {
        documentCalls += type
        return documentResult.getOrThrow()
    }
}
