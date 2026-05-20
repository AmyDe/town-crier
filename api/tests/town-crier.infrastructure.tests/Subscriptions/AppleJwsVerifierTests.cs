using TownCrier.Application.Subscriptions;
using TownCrier.Infrastructure.Subscriptions;

namespace TownCrier.Infrastructure.Tests.Subscriptions;

public sealed class AppleJwsVerifierTests
{
    private const string PayloadJson =
        "{\"transactionId\":\"txn-1\",\"bundleId\":\"uk.co.towncrier.ios\"}";

    [Test]
    public async Task Should_ReturnDecodedPayload_When_ChainIsValidAndSignatureMatches()
    {
        var builder = TestJwsBuilder.Create();
        var verifier = new AppleJwsVerifier([builder.RootCertificate]);
        var jws = builder.Sign(PayloadJson);

        var payload = await verifier.VerifyAndDecodeAsync(jws, CancellationToken.None);

        await Assert.That(payload).IsEqualTo(PayloadJson);
    }

    [Test]
    public async Task Should_Throw_When_SignatureIsTampered()
    {
        var builder = TestJwsBuilder.Create();
        var verifier = new AppleJwsVerifier([builder.RootCertificate]);
        var jws = builder.SignWithTamperedSignature(PayloadJson);

        await Assert.ThrowsAsync<AppleJwsVerificationException>(
            () => verifier.VerifyAndDecodeAsync(jws, CancellationToken.None));
    }

    [Test]
    public async Task Should_Throw_When_ChainDoesNotReachATrustedRoot()
    {
        var signingBuilder = TestJwsBuilder.Create();
        var unrelatedRoot = TestJwsBuilder.Create();
        var verifier = new AppleJwsVerifier([unrelatedRoot.RootCertificate]);
        var jws = signingBuilder.Sign(PayloadJson);

        await Assert.ThrowsAsync<AppleJwsVerificationException>(
            () => verifier.VerifyAndDecodeAsync(jws, CancellationToken.None));
    }

    [Test]
    public async Task Should_Throw_When_LeafCertificateIsExpired()
    {
        var builder = TestJwsBuilder.Create(
            leafNotBefore: DateTimeOffset.UtcNow.AddYears(-2),
            leafNotAfter: DateTimeOffset.UtcNow.AddYears(-1));
        var verifier = new AppleJwsVerifier([builder.RootCertificate]);
        var jws = builder.Sign(PayloadJson);

        await Assert.ThrowsAsync<AppleJwsVerificationException>(
            () => verifier.VerifyAndDecodeAsync(jws, CancellationToken.None));
    }

    [Test]
    public async Task Should_Throw_When_PayloadIsNotThreeParts()
    {
        var builder = TestJwsBuilder.Create();
        var verifier = new AppleJwsVerifier([builder.RootCertificate]);

        await Assert.ThrowsAsync<AppleJwsVerificationException>(
            () => verifier.VerifyAndDecodeAsync("not-a-jws", CancellationToken.None));
    }

    [Test]
    public async Task Should_Throw_When_HeaderHasNoCertificateChain()
    {
        var builder = TestJwsBuilder.Create();
        var verifier = new AppleJwsVerifier([builder.RootCertificate]);

        await Assert.ThrowsAsync<AppleJwsVerificationException>(
            () => verifier.VerifyAndDecodeAsync(
                "eyJhbGciOiJFUzI1NiJ9.eyJhIjoxfQ.c2ln", CancellationToken.None));
    }
}
