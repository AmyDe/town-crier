package uk.towncrierapp.domain

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

class DomainModuleTest {

    @Test
    fun `module name identifies the domain module`() {
        assertEquals("domain", DomainModule.NAME)
    }
}
