using System.Reflection;
using System.Security.Cryptography.X509Certificates;

namespace TownCrier.Infrastructure.Subscriptions;

/// <summary>
/// Loads the Apple root certificate(s) that Apple's StoreKit JWS payloads
/// chain to. The DER-encoded "Apple Root CA - G3" certificate is embedded as
/// an assembly resource so verification needs no filesystem or network access
/// and stays Native AOT-safe.
/// </summary>
public static class AppleRootCertificates
{
    private const string RootCertResourceName =
        "TownCrier.Infrastructure.Subscriptions.AppleRootCA-G3.cer";

    /// <summary>Loads the trusted Apple root certificate(s) for JWS chain validation.</summary>
    /// <returns>The Apple root certificate collection.</returns>
    public static IReadOnlyCollection<X509Certificate2> Load()
    {
        var assembly = Assembly.GetExecutingAssembly();

        using var stream = assembly.GetManifestResourceStream(RootCertResourceName)
            ?? throw new InvalidOperationException(
                $"Embedded Apple root certificate '{RootCertResourceName}' not found.");

        using var buffer = new MemoryStream();
        stream.CopyTo(buffer);

        return [X509CertificateLoader.LoadCertificate(buffer.ToArray())];
    }
}
