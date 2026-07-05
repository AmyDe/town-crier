package uk.towncrierapp.presentation.sharing

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/** Port of iOS `ShareURLTests` — same encoding rules, ported test-for-test. */
class ShareUrlTest {
    @Test
    fun `build returns the canonical share URL for a simple ref`() {
        val result = ShareUrl.build(authoritySlug = "camden", ref = "24/0001")

        assertEquals("https://share.towncrierapp.uk/a/camden/24/0001", result)
    }

    @Test
    fun `build preserves literal slashes in a slashed ref`() {
        val result = ShareUrl.build(authoritySlug = "kingston", ref = "Kingston/25/02755/CLC")

        assertEquals("https://share.towncrierapp.uk/a/kingston/Kingston/25/02755/CLC", result)
    }

    @Test
    fun `build percent-encodes a space in the ref`() {
        val result = ShareUrl.build(authoritySlug = "camden", ref = "24/0001 A")

        assertEquals("https://share.towncrierapp.uk/a/camden/24/0001%20A", result)
    }

    @Test
    fun `build percent-encodes a hash in the ref`() {
        val result = ShareUrl.build(authoritySlug = "camden", ref = "24/0001#A")

        assertEquals("https://share.towncrierapp.uk/a/camden/24/0001%23A", result)
    }

    @Test
    fun `build returns null for an empty authority slug`() {
        val result = ShareUrl.build(authoritySlug = "", ref = "24/0001")

        assertNull(result)
    }

    @Test
    fun `build returns null for an empty ref`() {
        val result = ShareUrl.build(authoritySlug = "camden", ref = "")

        assertNull(result)
    }

    @Test
    fun `origin is the public share domain`() {
        assertEquals("https://share.towncrierapp.uk", ShareUrl.ORIGIN)
    }
}
