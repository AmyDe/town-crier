package uk.towncrierapp.data

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

class DataModuleTest {

    @Test
    fun `module name identifies the data module`() {
        assertEquals("data", DataModule.NAME)
    }
}
