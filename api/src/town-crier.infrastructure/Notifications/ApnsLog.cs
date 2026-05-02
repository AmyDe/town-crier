using System.Globalization;
using Microsoft.Extensions.Logging;

namespace TownCrier.Infrastructure.Notifications;

internal static class ApnsLog
{
    private static readonly Action<ILogger, string, Exception?> TokenUnregisteredLog =
        LoggerMessage.Define<string>(LogLevel.Information, new EventId(1, nameof(TokenUnregistered)), "APNs token unregistered (410): {TokenPrefix}");

    private static readonly Action<ILogger, string, Exception?> BadDeviceTokenLog =
        LoggerMessage.Define<string>(LogLevel.Warning, new EventId(2, nameof(BadDeviceToken)), "APNs bad device token (400): {TokenPrefix}");

    private static readonly Action<ILogger, Exception?> ExpiredProviderTokenLog =
        LoggerMessage.Define(LogLevel.Information, new EventId(3, nameof(ExpiredProviderToken)), "APNs expired provider token (403); refreshing JWT and retrying once");

    private static readonly Action<ILogger, Exception?> TooManyProviderTokenUpdatesLog =
        LoggerMessage.Define(LogLevel.Warning, new EventId(4, nameof(TooManyProviderTokenUpdates)), "APNs returned 429 TooManyProviderTokenUpdates; deferring further mints");

    private static readonly Action<ILogger, int, string, int, Exception?> TransientErrorLog =
        LoggerMessage.Define<int, string, int>(LogLevel.Warning, new EventId(5, nameof(TransientError)), "APNs transient error: status={Status} reason={Reason} attempt={Attempt}");

    private static readonly Action<ILogger, int, string, string, Exception?> UnhandledStatusLog =
        LoggerMessage.Define<int, string, string>(LogLevel.Warning, new EventId(6, nameof(UnhandledStatus)), "APNs unhandled status: status={Status} reason={Reason} token={TokenPrefix}");

    private static readonly Action<ILogger, int, Exception?> HttpExceptionLog =
        LoggerMessage.Define<int>(LogLevel.Warning, new EventId(7, nameof(HttpException)), "APNs HTTP exception on attempt {Attempt}; backing off");

    private static readonly Action<ILogger, string, Exception?> SendFailedLog =
        LoggerMessage.Define<string>(LogLevel.Error, new EventId(8, nameof(SendFailed)), "APNs send failed for token {TokenPrefix}");

    public static void TokenUnregistered(ILogger logger, string token) => TokenUnregisteredLog(logger, Prefix(token), null);

    public static void BadDeviceToken(ILogger logger, string token) => BadDeviceTokenLog(logger, Prefix(token), null);

    public static void ExpiredProviderToken(ILogger logger) => ExpiredProviderTokenLog(logger, null);

    public static void TooManyProviderTokenUpdates(ILogger logger) => TooManyProviderTokenUpdatesLog(logger, null);

    public static void TransientError(ILogger logger, int status, string reason, int attempt) => TransientErrorLog(logger, status, reason, attempt, null);

    public static void UnhandledStatus(ILogger logger, int status, string reason, string token) => UnhandledStatusLog(logger, status, reason, Prefix(token), null);

    public static void HttpException(ILogger logger, int attempt, Exception ex) => HttpExceptionLog(logger, attempt, ex);

    public static void SendFailed(ILogger logger, string token, Exception ex) => SendFailedLog(logger, Prefix(token), ex);

    // Avoid logging the full APNs device token — it identifies a user's device.
    private static string Prefix(string token)
    {
        if (string.IsNullOrEmpty(token))
        {
            return string.Empty;
        }

        return token.Length <= 8
            ? string.Create(CultureInfo.InvariantCulture, $"{token[..^Math.Min(2, token.Length)]}...")
            : string.Create(CultureInfo.InvariantCulture, $"{token[..8]}...");
    }
}
