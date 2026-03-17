import Testing
import TownCrierDomain

@Suite("AppVersion")
struct AppVersionTests {
    @Test func init_parsesValidVersionString() {
        let version = AppVersion("1.2.3")

        #expect(version != nil)
        #expect(version?.major == 1)
        #expect(version?.minor == 2)
        #expect(version?.patch == 3)
    }

    @Test func init_returnsNilForInvalidString() {
        #expect(AppVersion("invalid") == nil)
        #expect(AppVersion("1.2") == nil)
        #expect(AppVersion("") == nil)
        #expect(AppVersion("1.2.3.4") == nil)
    }

    @Test func comparable_majorVersion() {
        let v1 = AppVersion(major: 1, minor: 0, patch: 0)
        let v2 = AppVersion(major: 2, minor: 0, patch: 0)

        #expect(v1 < v2)
        #expect(!(v2 < v1))
    }

    @Test func comparable_minorVersion() {
        let v1 = AppVersion(major: 1, minor: 1, patch: 0)
        let v2 = AppVersion(major: 1, minor: 2, patch: 0)

        #expect(v1 < v2)
    }

    @Test func comparable_patchVersion() {
        let v1 = AppVersion(major: 1, minor: 2, patch: 3)
        let v2 = AppVersion(major: 1, minor: 2, patch: 4)

        #expect(v1 < v2)
    }

    @Test func comparable_equalVersions() {
        let v1 = AppVersion(major: 1, minor: 0, patch: 0)
        let v2 = AppVersion(major: 1, minor: 0, patch: 0)

        #expect(v1 == v2)
        #expect(!(v1 < v2))
    }
}
