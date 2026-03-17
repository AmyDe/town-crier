namespace TownCrier.Domain.Designations;

public sealed class DesignationContext
{
    public DesignationContext(
        bool isWithinConservationArea,
        string? conservationAreaName,
        bool isWithinListedBuildingCurtilage,
        string? listedBuildingGrade,
        bool isWithinArticle4Area)
    {
        this.IsWithinConservationArea = isWithinConservationArea;
        this.ConservationAreaName = conservationAreaName;
        this.IsWithinListedBuildingCurtilage = isWithinListedBuildingCurtilage;
        this.ListedBuildingGrade = listedBuildingGrade;
        this.IsWithinArticle4Area = isWithinArticle4Area;
    }

    public static DesignationContext None { get; } = new(false, null, false, null, false);

    public bool IsWithinConservationArea { get; }

    public string? ConservationAreaName { get; }

    public bool IsWithinListedBuildingCurtilage { get; }

    public string? ListedBuildingGrade { get; }

    public bool IsWithinArticle4Area { get; }
}
