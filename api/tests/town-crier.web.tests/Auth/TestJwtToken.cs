using System.IdentityModel.Tokens.Jwt;
using System.Security.Claims;
using System.Security.Cryptography;
using Microsoft.IdentityModel.Tokens;

namespace TownCrier.Web.Tests.Auth;

internal static class TestJwtToken
{
    private static readonly RSA Rsa = RSA.Create(2048);

    internal static RsaSecurityKey SecurityKey { get; } = new(Rsa);

    internal static string Generate(string userId = "auth0|test-user-123", IEnumerable<Claim>? claims = null)
    {
        var credentials = new SigningCredentials(SecurityKey, SecurityAlgorithms.RsaSha256);
        var allClaims = new List<Claim> { new("sub", userId) };
        if (claims is not null)
        {
            allClaims.AddRange(claims);
        }

        var token = new JwtSecurityToken(
            issuer: "https://test.auth0.com/",
            audience: "https://api.towncrier.app",
            claims: allClaims,
            expires: DateTime.UtcNow.AddHours(1),
            signingCredentials: credentials);

        return new JwtSecurityTokenHandler().WriteToken(token);
    }
}
