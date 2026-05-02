namespace TownCrier.Infrastructure.Tests.Notifications;

internal sealed record RecordedRequest(
    HttpMethod Method,
    Uri? RequestUri,
    Version Version,
    HttpVersionPolicy VersionPolicy,
    RecordedAuth? Authorization,
    IReadOnlyDictionary<string, string> Headers,
    string Body);
