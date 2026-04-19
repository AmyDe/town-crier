using TownCrier.Application.Legal;

namespace TownCrier.Application.Tests.Legal;

public sealed class GetLegalDocumentQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnPrivacyPolicy_When_DocumentTypeIsPrivacy()
    {
        var query = new GetLegalDocumentQuery("privacy");

        var result = await GetLegalDocumentQueryHandler.HandleAsync(query, CancellationToken.None);

        await Assert.That(result).IsNotNull();
        await Assert.That(result!.DocumentType).IsEqualTo("privacy");
        await Assert.That(result.Title).IsEqualTo("Privacy Policy");
        await Assert.That(result.LastUpdated).IsEqualTo("2026-04-19");
        await Assert.That(result.Sections).HasCount().EqualTo(10);
        await Assert.That(result.Sections[0].Heading).IsEqualTo("Who We Are");
    }

    [Test]
    public async Task Should_ReturnTermsOfService_When_DocumentTypeIsTerms()
    {
        var query = new GetLegalDocumentQuery("terms");

        var result = await GetLegalDocumentQueryHandler.HandleAsync(query, CancellationToken.None);

        await Assert.That(result).IsNotNull();
        await Assert.That(result!.DocumentType).IsEqualTo("terms");
        await Assert.That(result.Title).IsEqualTo("Terms of Service");
        await Assert.That(result.LastUpdated).IsEqualTo("2026-03-16");
        await Assert.That(result.Sections).HasCount().EqualTo(6);
        await Assert.That(result.Sections[0].Heading).IsEqualTo("Acceptance of Terms");
    }

    [Test]
    public async Task Should_ReturnNull_When_DocumentTypeIsUnknown()
    {
        var query = new GetLegalDocumentQuery("unknown");

        var result = await GetLegalDocumentQueryHandler.HandleAsync(query, CancellationToken.None);

        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_BeCaseInsensitive_When_MatchingDocumentType()
    {
        var query = new GetLegalDocumentQuery("Privacy");

        var result = await GetLegalDocumentQueryHandler.HandleAsync(query, CancellationToken.None);

        await Assert.That(result).IsNotNull();
        await Assert.That(result!.DocumentType).IsEqualTo("privacy");
    }
}
