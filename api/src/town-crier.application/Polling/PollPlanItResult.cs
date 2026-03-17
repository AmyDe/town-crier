namespace TownCrier.Application.Polling;

public sealed record PollPlanItResult(int ApplicationCount, int AuthoritiesPolled = 0, int AuthoritiesSkipped = 0, int TotalActiveAuthorities = 0);
