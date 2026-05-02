using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

namespace TownCrier.Infrastructure.Notifications;

/// <summary>
/// Mints and caches an ES256-signed APNs provider JWT.
/// </summary>
/// <remarks>
/// Apple's window: a token older than 60 minutes returns 403 ExpiredProviderToken,
/// regenerating more often than once per 20 minutes returns 429 TooManyProviderTokenUpdates.
/// We refresh at ~50 minutes to stay safely inside both bounds. The provider is intended
/// to be a singleton — a single JWT is cached in memory and re-signed under a lock.
/// </remarks>
public sealed class ApnsJwtProvider : IDisposable
{
    private static readonly TimeSpan RefreshInterval = TimeSpan.FromMinutes(50);

    private readonly ECDsa key;
    private readonly string keyId;
    private readonly string teamId;
    private readonly TimeProvider timeProvider;
    private readonly Lock gate = new();

    private string? cached;
    private DateTimeOffset mintedAt;

    public ApnsJwtProvider(string privateKeyPem, string keyId, string teamId, TimeProvider timeProvider)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(privateKeyPem);
        ArgumentException.ThrowIfNullOrWhiteSpace(keyId);
        ArgumentException.ThrowIfNullOrWhiteSpace(teamId);
        ArgumentNullException.ThrowIfNull(timeProvider);

        this.key = ECDsa.Create();
        this.key.ImportFromPem(privateKeyPem);
        this.keyId = keyId;
        this.teamId = teamId;
        this.timeProvider = timeProvider;
    }

    /// <summary>
    /// Returns the current cached JWT, minting a fresh one if no JWT has been minted
    /// or the cached one is older than the refresh interval.
    /// </summary>
    public string Current()
    {
        lock (this.gate)
        {
            var now = this.timeProvider.GetUtcNow();
            if (this.cached is null || now - this.mintedAt > RefreshInterval)
            {
                this.cached = this.Mint(now);
                this.mintedAt = now;
            }

            return this.cached;
        }
    }

    public void Dispose()
    {
        this.key.Dispose();
    }

    private static string Base64UrlEncode(byte[] bytes)
    {
        // RFC 7515 base64url: '+' -> '-', '/' -> '_', strip '=' padding.
        return Convert.ToBase64String(bytes)
            .TrimEnd('=')
            .Replace('+', '-')
            .Replace('/', '_');
    }

    private string Mint(DateTimeOffset now)
    {
        var header = new ApnsJwtHeader("ES256", this.keyId);
        var payload = new ApnsJwtPayload(this.teamId, now.ToUnixTimeSeconds());

        var headerJson = JsonSerializer.SerializeToUtf8Bytes(header, ApnsJsonSerializerContext.Default.ApnsJwtHeader);
        var payloadJson = JsonSerializer.SerializeToUtf8Bytes(payload, ApnsJsonSerializerContext.Default.ApnsJwtPayload);

        var signingInput = $"{Base64UrlEncode(headerJson)}.{Base64UrlEncode(payloadJson)}";
        var signature = this.key.SignData(Encoding.UTF8.GetBytes(signingInput), HashAlgorithmName.SHA256);

        return $"{signingInput}.{Base64UrlEncode(signature)}";
    }
}
