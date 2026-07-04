package uk.towncrierapp.domain.legal

/**
 * Port for reading a bundled legal document. `:presentation` depends only
 * on this interface — the concrete asset-reading implementation
 * (`LegalDocumentLoader`) lives in `:data`, which `:presentation` is never
 * allowed to see (module dependency rule).
 */
public interface LegalDocumentRepository {
    /** Loads [type]'s bundled document. */
    public suspend fun document(type: LegalDocumentType): LegalDocument
}
