using System.Diagnostics.CodeAnalysis;

namespace TownCrier.Domain.Authorities;

public sealed class Authority
{
    [SuppressMessage("Design", "CA1054:URI parameters should not be strings", Justification = "URLs sourced as strings from external data")]
    public Authority(
        int id,
        string name,
        string areaType,
        string? councilUrl,
        string? planningUrl)
    {
        this.Id = id;
        this.Name = name;
        this.AreaType = areaType;
        this.CouncilUrl = councilUrl;
        this.PlanningUrl = planningUrl;
    }

    public int Id { get; }

    public string Name { get; }

    public string AreaType { get; }

    [SuppressMessage("Design", "CA1056:URI properties should not be strings", Justification = "URLs sourced as strings from external data")]
    public string? CouncilUrl { get; }

    [SuppressMessage("Design", "CA1056:URI properties should not be strings", Justification = "URLs sourced as strings from external data")]
    public string? PlanningUrl { get; }
}
