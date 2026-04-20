using TownCrier.Domain.OfferCodes;

namespace TownCrier.Application.OfferCodes;

public interface IOfferCodeRepository
{
    Task<OfferCode?> GetAsync(string canonicalCode, CancellationToken ct);

    // Inserts a new code. Throws if the code already exists (used to detect generator collisions).
    Task CreateAsync(OfferCode code, CancellationToken ct);

    Task SaveAsync(OfferCode code, CancellationToken ct);

    Task<IReadOnlyList<OfferCode>> GetRedeemedByUserIdAsync(string userId, CancellationToken ct);
}
