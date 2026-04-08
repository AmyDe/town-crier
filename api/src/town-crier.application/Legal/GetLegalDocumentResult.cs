namespace TownCrier.Application.Legal;

public sealed record GetLegalDocumentResult(
    string DocumentType,
    string Title,
    string LastUpdated,
    IReadOnlyList<LegalDocumentSectionResult> Sections);
