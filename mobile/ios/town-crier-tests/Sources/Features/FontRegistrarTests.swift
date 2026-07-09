import Testing

@testable import TownCrierPresentation

/// ``FontRegistrar`` registers the bundled Fraunces static instances with
/// Core Text at process startup (GH#857) — deliberately NOT via
/// `UIAppFonts`/`Info.plist`/`project.yml`, so the fonts ship as ordinary SPM
/// resources in `Bundle.module` and adding/changing one never touches the
/// generated `.xcodeproj`.
@Suite("FontRegistrar")
struct FontRegistrarTests {

  @Test func registerAll_registersBothBundledFraunces() {
    let results = FontRegistrar.registerAll()

    #expect(results["Fraunces-Regular"] == true)
    #expect(results["Fraunces-SemiBold"] == true)
  }

  @Test func registerAll_isSafeToCallMoreThanOnce() {
    _ = FontRegistrar.registerAll()
    let second = FontRegistrar.registerAll()

    #expect(second["Fraunces-Regular"] == true)
    #expect(second["Fraunces-SemiBold"] == true)
  }
}
