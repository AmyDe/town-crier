using TownCrier.Domain.Geocoding;

namespace TownCrier.Domain.Groups;

public sealed class Group
{
    private readonly List<GroupMember> members = [];

    private Group(
        string id,
        string name,
        string ownerId,
        Coordinates centre,
        double radiusMetres,
        int authorityId,
        DateTimeOffset createdAt)
    {
        this.Id = id;
        this.Name = name;
        this.OwnerId = ownerId;
        this.Centre = centre;
        this.RadiusMetres = radiusMetres;
        this.AuthorityId = authorityId;
        this.CreatedAt = createdAt;
    }

    public string Id { get; }

    public string Name { get; }

    public string OwnerId { get; }

    public Coordinates Centre { get; }

    public double RadiusMetres { get; }

    public int AuthorityId { get; }

    public DateTimeOffset CreatedAt { get; }

    public IReadOnlyList<GroupMember> Members => this.members;

    public static Group Create(
        string id,
        string name,
        string ownerId,
        Coordinates centre,
        double radiusMetres,
        int authorityId,
        DateTimeOffset now)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(id);
        ArgumentException.ThrowIfNullOrWhiteSpace(name);
        ArgumentException.ThrowIfNullOrWhiteSpace(ownerId);
        ArgumentNullException.ThrowIfNull(centre);
        ArgumentOutOfRangeException.ThrowIfNegativeOrZero(radiusMetres);
        ArgumentOutOfRangeException.ThrowIfNegativeOrZero(authorityId);

        var group = new Group(id, name, ownerId, centre, radiusMetres, authorityId, now);
        group.members.Add(GroupMember.CreateOwner(ownerId, now));
        return group;
    }

    public void AddMember(string userId, DateTimeOffset now)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);

        if (this.members.Exists(m => m.UserId == userId))
        {
            throw new InvalidOperationException($"User '{userId}' is already a member of this group.");
        }

        this.members.Add(GroupMember.CreateMember(userId, now));
    }

    public void RemoveMember(string requestingUserId, string memberUserId)
    {
        if (requestingUserId != this.OwnerId)
        {
            throw new UnauthorizedGroupOperationException("Only the group owner can remove members.");
        }

        if (memberUserId == this.OwnerId)
        {
            throw new InvalidOperationException("The group owner cannot be removed.");
        }

        var removed = this.members.RemoveAll(m => m.UserId == memberUserId);
        if (removed == 0)
        {
            throw new InvalidOperationException($"User '{memberUserId}' is not a member of this group.");
        }
    }

    public bool IsMember(string userId)
    {
        return this.members.Exists(m => m.UserId == userId);
    }

    internal static Group Reconstitute(
        string id,
        string name,
        string ownerId,
        Coordinates centre,
        double radiusMetres,
        int authorityId,
        DateTimeOffset createdAt,
        IEnumerable<GroupMember> members)
    {
        var group = new Group(id, name, ownerId, centre, radiusMetres, authorityId, createdAt);
        group.members.AddRange(members);
        return group;
    }
}
