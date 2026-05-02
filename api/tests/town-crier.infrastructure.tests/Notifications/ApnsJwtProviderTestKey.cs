using System.Security.Cryptography;

namespace TownCrier.Infrastructure.Tests.Notifications;

// Generates an ephemeral P-256 EC key in PKCS#8 PEM form for tests.
// Keys are created in-process — no real APNs auth keys are checked into the repo.
internal static class ApnsJwtProviderTestKey
{
    public static string GeneratePkcs8Pem()
    {
        using var ecdsa = ECDsa.Create(ECCurve.NamedCurves.nistP256);
        return ecdsa.ExportPkcs8PrivateKeyPem();
    }

    public static (string Pem, ECDsa PublicVerifier) GeneratePkcs8PemWithPublicVerifier()
    {
        using var ecdsa = ECDsa.Create(ECCurve.NamedCurves.nistP256);
        var pem = ecdsa.ExportPkcs8PrivateKeyPem();
        var publicVerifier = ECDsa.Create();
        publicVerifier.ImportSubjectPublicKeyInfo(ecdsa.ExportSubjectPublicKeyInfo(), out _);
        return (pem, publicVerifier);
    }
}
