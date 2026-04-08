namespace TownCrier.Application.Polling;

public sealed record PollPlanItResult(int ApplicationCount, int AuthoritiesPolled, bool RateLimited);
