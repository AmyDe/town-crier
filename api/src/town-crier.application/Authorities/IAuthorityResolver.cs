namespace TownCrier.Application.Authorities;

public interface IAuthorityResolver
{
    Task<int> ResolveFromCoordinatesAsync(double latitude, double longitude, CancellationToken ct);
}
