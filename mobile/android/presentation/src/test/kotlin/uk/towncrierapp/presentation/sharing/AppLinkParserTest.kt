package uk.towncrierapp.presentation.sharing

import uk.towncrierapp.domain.applications.PlanningApplicationId
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/** Port of iOS `UniversalLinkParserTests` — same test matrix, ported test-for-test (GH#782). */
class AppLinkParserTest {
    @Test
    fun `parse applicationDetail URL returns ApplicationDetail deep link`() {
        val result = AppLinkParser.parse("https://towncrierapp.uk/applications/19/00123/FUL")

        // path /applications/19/00123/FUL -> authority "19", name "00123/FUL"
        assertEquals(DeepLink.ApplicationDetail(PlanningApplicationId(authority = "19", name = "00123/FUL")), result)
    }

    @Test
    fun `parse applications root URL returns ApplicationsList deep link`() {
        val result = AppLinkParser.parse("https://towncrierapp.uk/applications")

        assertEquals(DeepLink.ApplicationsList, result)
    }

    @Test
    fun `parse applications root with trailing slash returns ApplicationsList deep link`() {
        val result = AppLinkParser.parse("https://towncrierapp.uk/applications/")

        assertEquals(DeepLink.ApplicationsList, result)
    }

    @Test
    fun `parse unrecognised path returns null`() {
        val result = AppLinkParser.parse("https://towncrierapp.uk/foo")

        assertNull(result)
    }

    @Test
    fun `parse empty path returns null`() {
        val result = AppLinkParser.parse("https://towncrierapp.uk")

        assertNull(result)
    }

    @Test
    fun `parse applications prefix without separator returns null`() {
        // Guard against false-positive matches like /applicationsfoo.
        val result = AppLinkParser.parse("https://towncrierapp.uk/applicationsfoo")

        assertNull(result)
    }

    // MARK: - Public share scheme /a/{authoritySlug}/{ref...} (GH#738 Slice 4 / GH#782)

    @Test
    fun `parse share URL with prefixed ref returns ShareApplication deep link`() {
        // The ref is the application's full area-prefixed PlanIt name, verbatim
        // - it contains slashes, which are preserved as-is after the slug segment.
        val result = AppLinkParser.parse("https://share.towncrierapp.uk/a/kingston/Kingston/25/02755/CLC")

        assertEquals(DeepLink.ShareApplication(authoritySlug = "kingston", ref = "Kingston/25/02755/CLC"), result)
    }

    @Test
    fun `parse share URL with simple ref returns ShareApplication deep link`() {
        val result = AppLinkParser.parse("https://share.towncrierapp.uk/a/croydon/23/03456/FUL")

        assertEquals(DeepLink.ShareApplication(authoritySlug = "croydon", ref = "23/03456/FUL"), result)
    }

    @Test
    fun `parse share prefix without separator returns null`() {
        // Guard against false-positive matches like /afoo: the /a/ separator is
        // required, so a path that merely starts with /a must not match.
        val result = AppLinkParser.parse("https://share.towncrierapp.uk/afoo")

        assertNull(result)
    }

    @Test
    fun `parse share bare path with no ref returns null`() {
        // /a carries neither a slug nor a ref.
        val result = AppLinkParser.parse("https://share.towncrierapp.uk/a")

        assertNull(result)
    }

    @Test
    fun `parse share trailing slash with no ref returns null`() {
        // /a/ has a separator but no slug/ref.
        val result = AppLinkParser.parse("https://share.towncrierapp.uk/a/")

        assertNull(result)
    }

    @Test
    fun `parse share slug without ref returns null`() {
        // A slug with no ref segment after it is not a valid share link.
        val result = AppLinkParser.parse("https://share.towncrierapp.uk/a/kingston")

        assertNull(result)
    }
}
