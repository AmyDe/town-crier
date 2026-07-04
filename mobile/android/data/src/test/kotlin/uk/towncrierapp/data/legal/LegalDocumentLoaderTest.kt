package uk.towncrierapp.data.legal

import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import uk.towncrierapp.domain.legal.LegalDocumentType
import java.time.LocalDate
import kotlin.test.assertEquals

private const val PRIVACY_JSON =
    """
    {
      "documentType": "privacy",
      "title": "Privacy Policy",
      "lastUpdated": "2026-07-01",
      "sections": [
        {"heading": "Who We Are", "body": "Town Crier is operated by Ivo and the Bea Ltd."},
        {"heading": "What We Collect", "body": "We keep the collection small."}
      ]
    }
    """

private const val TERMS_JSON =
    """
    {
      "documentType": "terms",
      "title": "Terms of Service",
      "lastUpdated": "2026-03-16",
      "sections": [
        {"heading": "Acceptance of Terms", "body": "By using Town Crier, you agree to these Terms of Service."}
      ]
    }
    """

/**
 * `LegalDocumentLoader` reads the bundled `legal/{privacy,terms}.json`
 * assets ({@code {title, lastUpdated: "YYYY-MM-DD", sections:[{heading,
 * body}]}}) via an injected [LegalDocumentAssetReader] — no Robolectric
 * needed to fake `AssetManager` (android-coding-standards skill: no
 * Robolectric).
 */
class LegalDocumentLoaderTest {
    private fun readerFor(vararg entries: Pair<String, String>): LegalDocumentAssetReader {
        val files = entries.toMap()
        return LegalDocumentAssetReader { fileName -> files.getValue(fileName) }
    }

    @Test
    fun `document PrivacyPolicy reads legal privacy json and parses every field`() =
        runTest {
            val sut = LegalDocumentLoader(readerFor("legal/privacy.json" to PRIVACY_JSON))

            val document = sut.document(LegalDocumentType.PRIVACY_POLICY)

            assertEquals("Privacy Policy", document.title)
            assertEquals(LocalDate.of(2026, 7, 1), document.lastUpdated)
            assertEquals(2, document.sections.size)
            assertEquals("Who We Are", document.sections[0].heading)
            assertEquals("Town Crier is operated by Ivo and the Bea Ltd.", document.sections[0].body)
        }

    @Test
    fun `document TermsOfService reads legal terms json`() =
        runTest {
            val sut = LegalDocumentLoader(readerFor("legal/terms.json" to TERMS_JSON))

            val document = sut.document(LegalDocumentType.TERMS_OF_SERVICE)

            assertEquals("Terms of Service", document.title)
            assertEquals(LocalDate.of(2026, 3, 16), document.lastUpdated)
            assertEquals(1, document.sections.size)
        }

    @Test
    fun `an unknown top-level field in the bundled JSON is ignored rather than crashing`() =
        runTest {
            // documentType is present in the bundled asset but not part of the domain shape.
            val sut = LegalDocumentLoader(readerFor("legal/privacy.json" to PRIVACY_JSON))

            sut.document(LegalDocumentType.PRIVACY_POLICY)
        }
}
