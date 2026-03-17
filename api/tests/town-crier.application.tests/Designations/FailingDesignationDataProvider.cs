using TownCrier.Application.Designations;
using TownCrier.Domain.Designations;

namespace TownCrier.Application.Tests.Designations;

internal sealed class FailingDesignationDataProvider : IDesignationDataProvider
{
    public Task<DesignationContext> GetDesignationsAsync(
        double latitude,
        double longitude,
        CancellationToken ct)
    {
        throw new HttpRequestException("Gov.uk Planning Data API is unavailable");
    }
}
