using TownCrier.Application.OfferCodes;
using TownCrier.Domain.OfferCodes;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.OfferCodes;

public sealed class CosmosOfferCodeRepository : IOfferCodeRepository
{
    private readonly ICosmosRestClient client;

    public CosmosOfferCodeRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<OfferCode?> GetAsync(string canonicalCode, CancellationToken ct)
    {
        var document = await this.client.ReadDocumentAsync(
            CosmosContainerNames.OfferCodes,
            canonicalCode,
            canonicalCode,
            CosmosJsonSerializerContext.Default.OfferCodeDocument,
            ct).ConfigureAwait(false);

        return document?.ToDomain();
    }

    // ICosmosRestClient.UpsertDocumentAsync has no distinct "create" semantics, so CreateAsync is
    // implemented as a best-effort upsert (last-writer-wins). See docs/specs/offer-codes.md §
    // Race-condition handling — ETag concurrency is deferred.
    public Task CreateAsync(OfferCode code, CancellationToken ct) => this.SaveAsync(code, ct);

    public async Task SaveAsync(OfferCode code, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(code);

        var document = OfferCodeDocument.FromDomain(code);
        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.OfferCodes,
            document,
            document.Id,
            CosmosJsonSerializerContext.Default.OfferCodeDocument,
            ct).ConfigureAwait(false);
    }
}
