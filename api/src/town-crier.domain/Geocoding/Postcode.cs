using System.Text.RegularExpressions;

namespace TownCrier.Domain.Geocoding;

public sealed partial class Postcode
{
    private Postcode(string value)
    {
        this.Value = value;
    }

    public string Value { get; }

    public static Postcode Create(string raw)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(raw);

        var normalised = raw.Trim().ToUpperInvariant();

        if (!UkPostcodeRegex().IsMatch(normalised))
        {
            throw new ArgumentException($"'{raw}' is not a valid UK postcode.", nameof(raw));
        }

        return new Postcode(normalised);
    }

    public override string ToString() => this.Value;

    [GeneratedRegex(@"^[A-Z]{1,2}\d[A-Z\d]?\s?\d[A-Z]{2}$")]
    private static partial Regex UkPostcodeRegex();
}
