using System.Globalization;

namespace TownCrier.Application.Polling;

/// <summary>
/// Parses HTTP <c>Retry-After</c> header values. Supports both the delta-seconds
/// form ("120") and the HTTP-date form ("Wed, 21 Oct 2015 07:28:00 GMT").
/// Malformed, negative, or missing values return <c>null</c> so callers can fall
/// back to a default policy. Past HTTP-dates clamp to <see cref="TimeSpan.Zero"/>.
/// </summary>
public static class RetryAfterParser
{
    public static TimeSpan? Parse(string? header, DateTimeOffset now)
    {
        if (string.IsNullOrWhiteSpace(header))
        {
            return null;
        }

        if (int.TryParse(header, NumberStyles.Integer, CultureInfo.InvariantCulture, out var seconds))
        {
            if (seconds < 0)
            {
                return null;
            }

            return TimeSpan.FromSeconds(seconds);
        }

        if (DateTimeOffset.TryParse(
                header,
                CultureInfo.InvariantCulture,
                DateTimeStyles.AssumeUniversal | DateTimeStyles.AdjustToUniversal,
                out var target))
        {
            var delta = target - now;
            return delta < TimeSpan.Zero ? TimeSpan.Zero : delta;
        }

        return null;
    }
}
