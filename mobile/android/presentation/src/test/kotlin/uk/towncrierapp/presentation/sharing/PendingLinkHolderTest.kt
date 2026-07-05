package uk.towncrierapp.presentation.sharing

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * A cold-start App Link must NOT resolve while the user is signed out (no
 * signed-out detail view day-1, GH#782 pre-resolved decision) - it is held
 * until authentication completes, then dispatched exactly once. iOS parity:
 * the by-slug read is anonymous, but the app experience is authed.
 */
class PendingLinkHolderTest {
    private val link = DeepLink.ShareApplication(authoritySlug = "camden", ref = "24/0001")

    @Test
    fun `a link received while signed out is held, not surfaced`() {
        val holder = PendingLinkHolder()

        holder.linkReceived(link)

        assertNull(holder.readyLink.value)
    }

    @Test
    fun `a held link is surfaced the moment authentication completes`() {
        val holder = PendingLinkHolder()
        holder.linkReceived(link)

        holder.onAuthenticationChanged(authenticated = true)

        assertEquals(link, holder.readyLink.value)
    }

    @Test
    fun `a link received while already signed in is surfaced immediately`() {
        val holder = PendingLinkHolder()
        holder.onAuthenticationChanged(authenticated = true)

        holder.linkReceived(link)

        assertEquals(link, holder.readyLink.value)
    }

    @Test
    fun `consume clears the ready link so it is only dispatched once`() {
        val holder = PendingLinkHolder()
        holder.onAuthenticationChanged(authenticated = true)
        holder.linkReceived(link)

        holder.consume()

        assertNull(holder.readyLink.value)
    }

    @Test
    fun `signing out after holding a link does not surface it retroactively`() {
        val holder = PendingLinkHolder()
        holder.linkReceived(link)

        holder.onAuthenticationChanged(authenticated = false)

        assertNull(holder.readyLink.value)
    }

    @Test
    fun `a second link received while authenticated replaces the first`() {
        val holder = PendingLinkHolder()
        holder.onAuthenticationChanged(authenticated = true)
        val second = DeepLink.ShareApplication(authoritySlug = "croydon", ref = "23/03456/FUL")

        holder.linkReceived(link)
        holder.linkReceived(second)

        assertEquals(second, holder.readyLink.value)
    }
}
