using TownCrier.Domain.Designations;

namespace TownCrier.Application.Designations;

public interface IDesignationDataProvider
{
    Task<DesignationContext> GetDesignationsAsync(
        double latitude,
        double longitude,
        CancellationToken ct);
}
