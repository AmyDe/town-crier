using Azure.Identity;
using TownCrier.Infrastructure.Identity;

namespace TownCrier.Infrastructure.Tests.Identity;

public sealed class ManagedIdentityCredentialFactoryTests
{
    [Test]
    public async Task Should_ReturnManagedIdentityCredential_When_ClientIdProvided()
    {
        var credential = ManagedIdentityCredentialFactory.Create("11111111-2222-3333-4444-555555555555");

        await Assert.That(credential).IsTypeOf<ManagedIdentityCredential>();
    }

    [Test]
    public async Task Should_ReturnManagedIdentityCredential_When_ClientIdIsNull()
    {
        var credential = ManagedIdentityCredentialFactory.Create(null);

        await Assert.That(credential).IsTypeOf<ManagedIdentityCredential>();
    }

    [Test]
    public async Task Should_ReturnManagedIdentityCredential_When_ClientIdIsWhitespace()
    {
        var credential = ManagedIdentityCredentialFactory.Create("   ");

        await Assert.That(credential).IsTypeOf<ManagedIdentityCredential>();
    }
}
