package uk.towncrierapp.data.legal

import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import uk.towncrierapp.domain.legal.LegalDocument
import uk.towncrierapp.domain.legal.LegalDocumentRepository
import uk.towncrierapp.domain.legal.LegalDocumentSection
import uk.towncrierapp.domain.legal.LegalDocumentType
import java.time.LocalDate

/**
 * `LegalDocumentRepository` over the bundled `legal/{privacy,terms}.json`
 * assets. iOS NEVER fetches the legal document endpoints at runtime and
 * neither does Android (epic #770 pre-resolved decision): offline, no loading state,
 * byte-mirrored from `api-go/internal/legal/resources/` by
 * `scripts/sync-legal.sh`, CI-enforced by `scripts/check-legal-drift.sh`.
 * Port of iOS `LegalDocumentViewModel`'s bundle-decoding half.
 */
public class LegalDocumentLoader(
    private val assetReader: LegalDocumentAssetReader,
    private val json: Json = Json { ignoreUnknownKeys = true },
    private val io: CoroutineDispatcher = Dispatchers.IO,
) : LegalDocumentRepository {
    override suspend fun document(type: LegalDocumentType): LegalDocument =
        withContext(io) {
            val raw = assetReader.read("legal/${type.assetFileName()}")
            json.decodeFromString(LegalDocumentJsonDto.serializer(), raw).toDomain()
        }
}

private fun LegalDocumentType.assetFileName(): String =
    when (this) {
        LegalDocumentType.PRIVACY_POLICY -> "privacy.json"
        LegalDocumentType.TERMS_OF_SERVICE -> "terms.json"
    }

// MARK: - JSON decoding — {title, lastUpdated: "YYYY-MM-DD", sections:[{heading, body}]}.
// `documentType` is present in the bundled asset but not part of this shape;
// ignoreUnknownKeys absorbs it.

@Serializable
internal data class LegalDocumentJsonDto(
    val title: String,
    val lastUpdated: String,
    val sections: List<LegalDocumentSectionDto>,
)

@Serializable
internal data class LegalDocumentSectionDto(
    val heading: String,
    val body: String,
)

internal fun LegalDocumentJsonDto.toDomain(): LegalDocument =
    LegalDocument(
        title = title,
        lastUpdated = LocalDate.parse(lastUpdated),
        sections = sections.map { LegalDocumentSection(heading = it.heading, body = it.body) },
    )
