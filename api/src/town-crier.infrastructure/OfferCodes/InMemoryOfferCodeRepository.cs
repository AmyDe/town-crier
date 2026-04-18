using System.Collections.Concurrent;
using TownCrier.Application.OfferCodes;
using TownCrier.Domain.OfferCodes;

namespace TownCrier.Infrastructure.OfferCodes;

public sealed class InMemoryOfferCodeRepository : IOfferCodeRepository
{
    private readonly ConcurrentDictionary<string, OfferCode> store = new(StringComparer.Ordinal);

    public Task<OfferCode?> GetAsync(string canonicalCode, CancellationToken ct)
    {
        this.store.TryGetValue(canonicalCode, out var code);
        return Task.FromResult(code);
    }

    public Task CreateAsync(OfferCode code, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(code);

        if (!this.store.TryAdd(code.Code, code))
        {
            throw new InvalidOperationException($"Offer code '{code.Code}' already exists.");
        }

        return Task.CompletedTask;
    }

    public Task SaveAsync(OfferCode code, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(code);
        this.store[code.Code] = code;
        return Task.CompletedTask;
    }
}
