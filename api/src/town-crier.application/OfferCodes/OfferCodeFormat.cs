namespace TownCrier.Application.OfferCodes;

public static class OfferCodeFormat
{
    public const int CanonicalLength = 12;
    public const string Alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ";

    public static string Normalize(string input)
    {
        if (string.IsNullOrWhiteSpace(input))
        {
            throw new InvalidOfferCodeFormatException("Offer code is required.");
        }

        Span<char> buffer = stackalloc char[input.Length];
        var length = 0;

        foreach (var c in input)
        {
            if (c == '-' || char.IsWhiteSpace(c))
            {
                continue;
            }

            var upper = char.ToUpperInvariant(c);
            if (Alphabet.IndexOf(upper, StringComparison.Ordinal) < 0)
            {
                throw new InvalidOfferCodeFormatException(
                    $"Offer code contains invalid character '{c}'.");
            }

            buffer[length++] = upper;
        }

        if (length != CanonicalLength)
        {
            throw new InvalidOfferCodeFormatException(
                $"Offer code must be {CanonicalLength} characters (got {length}).");
        }

        return new string(buffer[..length]);
    }

    public static string Format(string canonical)
    {
        ArgumentNullException.ThrowIfNull(canonical);

        if (!IsValidCanonical(canonical))
        {
            throw new ArgumentException("Expected canonical 12-char code.", nameof(canonical));
        }

        return $"{canonical[..4]}-{canonical.Substring(4, 4)}-{canonical[8..]}";
    }

    public static bool IsValidCanonical(string? value)
    {
        if (value is null || value.Length != CanonicalLength)
        {
            return false;
        }

        foreach (var c in value)
        {
            if (Alphabet.IndexOf(c, StringComparison.Ordinal) < 0)
            {
                return false;
            }
        }

        return true;
    }
}
