using System.Text.Json;

namespace TownCrier.Application.Legal;

public static class GetLegalDocumentQueryHandler
{
    private static readonly Lazy<GetLegalDocumentResult> PrivacyPolicy =
        new(() => LoadDocument("privacy.json"));

    private static readonly Lazy<GetLegalDocumentResult> TermsOfService =
        new(() => LoadDocument("terms.json"));

    public static Task<GetLegalDocumentResult?> HandleAsync(
        GetLegalDocumentQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        GetLegalDocumentResult? result = query.DocumentType.ToUpperInvariant() switch
        {
            "PRIVACY" => PrivacyPolicy.Value,
            "TERMS" => TermsOfService.Value,
            _ => null,
        };

        return Task.FromResult(result);
    }

    private static GetLegalDocumentResult LoadDocument(string fileName)
    {
        var assembly = typeof(GetLegalDocumentQueryHandler).Assembly;
        var resourceName = $"TownCrier.Application.Legal.Resources.{fileName}";

        using var stream = assembly.GetManifestResourceStream(resourceName)
            ?? throw new InvalidOperationException(
                $"Embedded legal document resource not found: {resourceName}");

        return JsonSerializer.Deserialize(
                stream,
                LegalDocumentJsonSerializerContext.Default.GetLegalDocumentResult)
            ?? throw new InvalidOperationException(
                $"Failed to deserialize legal document: {resourceName}");
    }
}
