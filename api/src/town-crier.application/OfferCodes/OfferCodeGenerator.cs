using System.Security.Cryptography;

namespace TownCrier.Application.OfferCodes;

public sealed class OfferCodeGenerator : IOfferCodeGenerator
{
    public string Generate()
    {
        // 12 characters × 5 bits = 60 bits. Use 8 bytes (64 bits) and discard the top 4.
        Span<byte> randomBytes = stackalloc byte[8];
        RandomNumberGenerator.Fill(randomBytes);

        var value = BitConverter.ToUInt64(randomBytes);

        Span<char> buffer = stackalloc char[OfferCodeFormat.CanonicalLength];
        for (var i = OfferCodeFormat.CanonicalLength - 1; i >= 0; i--)
        {
            buffer[i] = OfferCodeFormat.Alphabet[(int)(value & 0x1F)];
            value >>= 5;
        }

        return new string(buffer);
    }
}
