package uk.towncrierapp.app

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

/**
 * Because the Android leaves are constructor parameters, a plain JVM test
 * can construct the whole composition root — wiring drift fails here, in
 * `./gradlew test`, rather than on first launch (android-coding-standards
 * skill, architecture-and-modules.md).
 */
class AppGraphSmokeTest {
    @Test
    fun `constructs the composition root without throwing`() {
        val graph = AppGraph(baseUrl = "https://api-dev.towncrierapp.uk")

        assertEquals("https://api-dev.towncrierapp.uk", graph.baseUrl)
    }
}
