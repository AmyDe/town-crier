using TownCrier.Domain.Groups;

namespace TownCrier.Application.Groups;

public interface IGroupRepository
{
    Task<Group?> GetByIdAsync(string groupId, CancellationToken ct);

    Task SaveAsync(Group group, CancellationToken ct);

    Task DeleteAsync(string groupId, CancellationToken ct);

    Task<IReadOnlyList<Group>> GetByUserIdAsync(string userId, CancellationToken ct);
}
