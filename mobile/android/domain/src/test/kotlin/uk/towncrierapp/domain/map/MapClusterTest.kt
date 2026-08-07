package uk.towncrierapp.domain.map

import org.junit.jupiter.api.Test
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.aPlanningApplicationId
import uk.towncrierapp.domain.watchzones.aCoordinate
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** [MapCluster]'s three shapes — single pin, splittable bubble, unsplittable (stacked) bubble — port of iOS `MapClusterTests`. */
class MapClusterTest {
    @Test
    fun `a single-member cluster is a single member and never stacked`() {
        val cluster = aSingleMemberMapCluster()

        assertTrue(cluster.isSingleMember)
        assertFalse(cluster.isStacked)
    }

    @Test
    fun `a splittable bubble is neither a single member nor stacked`() {
        val cluster = aSplittableMapCluster(count = 5)

        assertFalse(cluster.isSingleMember)
        assertFalse(cluster.isStacked)
    }

    @Test
    fun `a stacked cluster is stacked, never a single member`() {
        val cluster = aStackedMapCluster()

        assertFalse(cluster.isSingleMember)
        assertTrue(cluster.isStacked)
    }

    @Test
    fun `memberStatus is the lone status only for a single-member cluster`() {
        val single = aSingleMemberMapCluster(status = ApplicationStatus.Permitted)
        val bubble = aSplittableMapCluster()

        assertEquals(ApplicationStatus.Permitted, single.memberStatus)
        assertNull(bubble.memberStatus)
    }

    @Test
    fun `id is the member's value for a single-member cluster`() {
        val id = aPlanningApplicationId(authority = "42", name = "24/0001")
        val cluster = aSingleMemberMapCluster(member = id)

        assertEquals("42/24/0001", cluster.id)
    }

    @Test
    fun `id is a centroid-and-count composite for a bubble or stacked cluster`() {
        val coordinate = aCoordinate(latitude = 51.5, longitude = -0.1)
        val cluster = aSplittableMapCluster(coordinate = coordinate, count = 7)

        assertEquals("cluster:51.5:-0.1:7", cluster.id)
    }

    @Test
    fun `count must be positive`() {
        assertFailsWith<IllegalArgumentException> {
            aSplittableMapCluster(count = 0)
        }
    }
}
