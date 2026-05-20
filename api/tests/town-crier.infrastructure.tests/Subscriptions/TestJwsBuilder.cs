using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text;
using System.Text.Json;

namespace TownCrier.Infrastructure.Tests.Subscriptions;

/// <summary>
/// Test helper that builds a self-contained EC certificate chain
/// (root -> intermediate -> leaf) and signs an ES256 JWS the same way
/// Apple's StoreKit infrastructure does — an <c>x5c</c> header carrying
/// the chain (leaf first) and an ES256 signature over <c>header.payload</c>.
/// </summary>
internal sealed class TestJwsBuilder
{
    private readonly X509Certificate2 root;
    private readonly X509Certificate2 intermediate;
    private readonly X509Certificate2 leaf;
    private readonly ECDsa leafKey;

    private TestJwsBuilder(
        X509Certificate2 root,
        X509Certificate2 intermediate,
        X509Certificate2 leaf,
        ECDsa leafKey)
    {
        this.root = root;
        this.intermediate = intermediate;
        this.leaf = leaf;
        this.leafKey = leafKey;
    }

    /// <summary>Gets the root certificate — pass to the verifier as a trusted root.</summary>
    public X509Certificate2 RootCertificate => this.root;

    public static TestJwsBuilder Create(
        DateTimeOffset? leafNotBefore = null,
        DateTimeOffset? leafNotAfter = null)
    {
        var now = DateTimeOffset.UtcNow;

        var rootKey = ECDsa.Create(ECCurve.NamedCurves.nistP256);
        var rootReq = new CertificateRequest(
            "CN=Test Apple Root CA", rootKey, HashAlgorithmName.SHA256);
        rootReq.CertificateExtensions.Add(
            new X509BasicConstraintsExtension(true, false, 0, true));
        var root = rootReq.CreateSelfSigned(now.AddYears(-1), now.AddYears(10));
        var rootGenerator = X509SignatureGenerator.CreateForECDsa(rootKey);

        var intermediateKey = ECDsa.Create(ECCurve.NamedCurves.nistP256);
        var intermediateReq = new CertificateRequest(
            "CN=Test Apple Intermediate CA", intermediateKey, HashAlgorithmName.SHA256);
        intermediateReq.CertificateExtensions.Add(
            new X509BasicConstraintsExtension(true, false, 0, true));
        var intermediate = intermediateReq.Create(
            root.SubjectName,
            rootGenerator,
            now.AddYears(-1),
            now.AddYears(5),
            Guid.NewGuid().ToByteArray());
        var intermediateGenerator = X509SignatureGenerator.CreateForECDsa(intermediateKey);

        var leafKey = ECDsa.Create(ECCurve.NamedCurves.nistP256);
        var leafReq = new CertificateRequest(
            "CN=Test Apple Leaf", leafKey, HashAlgorithmName.SHA256);
        leafReq.CertificateExtensions.Add(
            new X509BasicConstraintsExtension(false, false, 0, true));
        var leaf = leafReq.Create(
            intermediate.SubjectName,
            intermediateGenerator,
            leafNotBefore ?? now.AddDays(-1),
            leafNotAfter ?? now.AddYears(1),
            Guid.NewGuid().ToByteArray());

        rootKey.Dispose();
        intermediateKey.Dispose();

        return new TestJwsBuilder(root, intermediate, leaf, leafKey);
    }

    // Signs the payload as a valid ES256 JWS carrying the test certificate chain.
    public string Sign(string payloadJson)
    {
        var x5c = new[]
        {
            Convert.ToBase64String(this.leaf.RawData),
            Convert.ToBase64String(this.intermediate.RawData),
            Convert.ToBase64String(this.root.RawData),
        };

        var headerJson = JsonSerializer.Serialize(
            new JwsTestHeader("ES256", x5c),
            TestJwsJsonContext.Default.JwsTestHeader);

        var encodedHeader = Base64UrlEncode(Encoding.UTF8.GetBytes(headerJson));
        var encodedPayload = Base64UrlEncode(Encoding.UTF8.GetBytes(payloadJson));
        var signingInput = $"{encodedHeader}.{encodedPayload}";

        var signature = this.leafKey.SignData(
            Encoding.ASCII.GetBytes(signingInput),
            HashAlgorithmName.SHA256,
            DSASignatureFormat.IeeeP1363FixedFieldConcatenation);

        return $"{signingInput}.{Base64UrlEncode(signature)}";
    }

    // Signs the payload then flips one bit of the signature — a tampered JWS.
    public string SignWithTamperedSignature(string payloadJson)
    {
        var jws = this.Sign(payloadJson);
        var parts = jws.Split('.');
        var signature = Base64UrlDecode(parts[2]);
        signature[0] ^= 0xFF;
        return $"{parts[0]}.{parts[1]}.{Base64UrlEncode(signature)}";
    }

    private static string Base64UrlEncode(byte[] bytes) =>
        Convert.ToBase64String(bytes)
            .TrimEnd('=')
            .Replace('+', '-')
            .Replace('/', '_');

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
}
