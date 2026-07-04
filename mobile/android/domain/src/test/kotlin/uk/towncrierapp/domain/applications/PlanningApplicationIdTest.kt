package uk.towncrierapp.domain.applications

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

/**
 * Canonical `"authority/name"` encoding/decoding. [PlanningApplicationId.parse]
 * splits on the FIRST `/` only, because [PlanningApplicationId.name] (the
 * PlanIt case reference) routinely contains further slashes itself, e.g.
 * `"24/0001"` (GH#775).
 */
class PlanningApplicationIdTest {
    @Test
    fun `value joins authority and name with a slash`() {
        val id = PlanningApplicationId(authority = "42", name = "24/0001")

        assertEquals("42/24/0001", id.value)
    }

    @Test
    fun `parse splits on the first slash only, keeping further slashes in name`() {
        val id = PlanningApplicationId.parse("42/24/0001/FUL")

        assertEquals("42", id.authority)
        assertEquals("24/0001/FUL", id.name)
    }

    @Test
    fun `parse is the inverse of value`() {
        val id = PlanningApplicationId(authority = "7", name = "S/24/0099")

        assertEquals(id, PlanningApplicationId.parse(id.value))
    }
}
