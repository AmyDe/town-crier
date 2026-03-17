using TownCrier.Domain.Authorities;

namespace TownCrier.Application.Authorities;

public interface IAuthorityProvider
{
    Task<IReadOnlyList<Authority>> GetAllAsync(CancellationToken ct);

    Task<Authority?> GetByIdAsync(int id, CancellationToken ct);
}
