namespace TownCrier.Web.Observability;

internal sealed record ErrorResponse(int Status, string Title, string CorrelationId);
