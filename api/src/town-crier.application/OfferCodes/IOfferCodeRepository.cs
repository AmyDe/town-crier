using TownCrier.Domain.OfferCodes;

namespace TownCrier.Application.OfferCodes;

public interface IOfferCodeRepository
{
    Task<OfferCode?> GetAsync(string canonicalCode, CancellationToken ct);

    // Inserts a new code. Throws if the code already exists (used to detect generator collisions).
    Task CreateAsync(OfferCode code, CancellationToken ct);

    Task SaveAsync(OfferCode code, CancellationToken ct);

    // Cross-partition — offer codes are partitioned by code; finding by redeemer requires
    // scanning all partitions. Used by ExportUserDataQueryHandler (GDPR export, user-initiated
    // but explicitly accepted in GH#395 as low-frequency per-user data export).
    Task<IReadOnlyList<OfferCode>> GetRedeemedByUserIdCrossPartitionAsync(string userId, CancellationToken ct);
}
