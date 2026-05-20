using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text;
using System.Text.Json;
using TownCrier.Application.Subscriptions;

namespace TownCrier.Infrastructure.Subscriptions;

/// <summary>
/// Verifies an Apple StoreKit JWS (JSON Web Signature) without any network
/// call to Apple, per ADR 0010. The JWS protected header carries an <c>x5c</c>
/// certificate chain (leaf first); the chain is validated against the
/// configured trusted root(s), then the ES256 signature is verified with the
/// leaf certificate's public key.
/// </summary>
/// <remarks>
/// Native AOT-safe: uses only <see cref="System.Security.Cryptography"/>
/// primitives and System.Text.Json source generation — no reflection.
/// </remarks>
public sealed class AppleJwsVerifier : IAppleJwsVerifier
{
    private readonly X509Certificate2Collection trustedRoots;

    public AppleJwsVerifier(IReadOnlyCollection<X509Certificate2> trustedRoots)
    {
        ArgumentNullException.ThrowIfNull(trustedRoots);

        if (trustedRoots.Count == 0)
        {
            throw new ArgumentException(
                "At least one trusted root certificate is required.", nameof(trustedRoots));
        }

        this.trustedRoots = [.. trustedRoots];
    }

    public Task<string> VerifyAndDecodeAsync(string signedPayload, CancellationToken ct)
    {
        if (string.IsNullOrWhiteSpace(signedPayload))
        {
            throw new AppleJwsVerificationException("The signed payload is empty.");
        }

        var parts = signedPayload.Split('.');
        if (parts.Length != 3)
        {
            throw new AppleJwsVerificationException(
                "The signed payload is not a JWS compact serialization (expected three parts).");
        }

        var header = ParseHeader(parts[0]);
        var chain = ParseCertificateChain(header);
        var leaf = chain[0];

        this.VerifyChainTrust(chain);
        VerifySignature(leaf, parts[0], parts[1], parts[2]);

        var payloadJson = Encoding.UTF8.GetString(Base64UrlDecode(parts[1]));
        return Task.FromResult(payloadJson);
    }

    private static JwsHeader ParseHeader(string encodedHeader)
    {
        try
        {
            var headerJson = Base64UrlDecode(encodedHeader);
            var header = JsonSerializer.Deserialize(
                headerJson, SubscriptionsJsonSerializerContext.Default.JwsHeader);

            return header ?? throw new AppleJwsVerificationException("The JWS header is empty.");
        }
        catch (JsonException ex)
        {
            throw new AppleJwsVerificationException("The JWS header is not valid JSON.", ex);
        }
        catch (FormatException ex)
        {
            throw new AppleJwsVerificationException("The JWS header is not valid base64url.", ex);
        }
    }

    private static List<X509Certificate2> ParseCertificateChain(JwsHeader header)
    {
        if (header.X5c is null || header.X5c.Length == 0)
        {
            throw new AppleJwsVerificationException(
                "The JWS header does not contain an x5c certificate chain.");
        }

        if (!string.Equals(header.Alg, "ES256", StringComparison.Ordinal))
        {
            throw new AppleJwsVerificationException(
                $"Unsupported JWS algorithm '{header.Alg}'. Apple App Store payloads use ES256.");
        }

        var chain = new List<X509Certificate2>(header.X5c.Length);
        try
        {
            foreach (var encodedCert in header.X5c)
            {
                chain.Add(X509CertificateLoader.LoadCertificate(Convert.FromBase64String(encodedCert)));
            }
        }
        catch (FormatException ex)
        {
            throw new AppleJwsVerificationException(
                "An x5c entry is not valid base64.", ex);
        }
        catch (CryptographicException ex)
        {
            throw new AppleJwsVerificationException(
                "An x5c entry is not a valid X.509 certificate.", ex);
        }

        return chain;
    }

    private static void VerifySignature(
        X509Certificate2 leaf, string encodedHeader, string encodedPayload, string encodedSignature)
    {
        using var publicKey = leaf.GetECDsaPublicKey()
            ?? throw new AppleJwsVerificationException(
                "The leaf certificate does not contain an ECDSA public key.");

        byte[] signature;
        try
        {
            signature = Base64UrlDecode(encodedSignature);
        }
        catch (FormatException ex)
        {
            throw new AppleJwsVerificationException(
                "The JWS signature is not valid base64url.", ex);
        }

        var signingInput = Encoding.ASCII.GetBytes($"{encodedHeader}.{encodedPayload}");

        var valid = publicKey.VerifyData(
            signingInput,
            signature,
            HashAlgorithmName.SHA256,
            DSASignatureFormat.IeeeP1363FixedFieldConcatenation);

        if (!valid)
        {
            throw new AppleJwsVerificationException(
                "The JWS signature does not match the payload.");
        }
    }

    private static byte[] Base64UrlDecode(string value)
    {
        var padded = value.Replace('-', '+').Replace('_', '/');
        switch (padded.Length % 4)
        {
            case 2: padded += "=="; break;
            case 3: padded += "="; break;
            default: break;
        }

        return Convert.FromBase64String(padded);
    }

    private void VerifyChainTrust(List<X509Certificate2> chain)
    {
        using var x509Chain = new X509Chain();
        x509Chain.ChainPolicy.RevocationMode = X509RevocationMode.NoCheck;
        x509Chain.ChainPolicy.TrustMode = X509ChainTrustMode.CustomRootTrust;
        x509Chain.ChainPolicy.CustomTrustStore.AddRange(this.trustedRoots);

        // Intermediate certificates travel in the x5c header, not a system
        // store; supply them explicitly so the chain can be built.
        for (var i = 1; i < chain.Count; i++)
        {
            x509Chain.ChainPolicy.ExtraStore.Add(chain[i]);
        }

        if (!x509Chain.Build(chain[0]))
        {
            var reasons = string.Join(
                ", ", x509Chain.ChainStatus.Select(s => s.StatusInformation.Trim()));
            throw new AppleJwsVerificationException(
                $"The certificate chain did not validate to a trusted Apple root: {reasons}");
        }
    }
}
