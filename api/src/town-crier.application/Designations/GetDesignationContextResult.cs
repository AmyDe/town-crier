namespace TownCrier.Application.Designations;

public sealed record GetDesignationContextResult(
    bool IsWithinConservationArea,
    string? ConservationAreaName,
    bool IsWithinListedBuildingCurtilage,
    string? ListedBuildingGrade,
    bool IsWithinArticle4Area);
