namespace TownCrier.Application.Polling;

/// <summary>
/// Outcome of <see cref="PollTriggerBootstrapper.TryBootstrapAsync"/>.
/// </summary>
/// <param name="Published">Whether a bootstrap trigger was successfully published.</param>
/// <param name="ProbeFailed">Whether the probe or publish threw (absorbed). Useful for
/// telemetry — a failed bootstrap is not a worker failure, but is worth recording.</param>
public sealed record PollTriggerBootstrapResult(bool Published, bool ProbeFailed);
