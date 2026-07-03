package uk.towncrierapp.app

import java.time.Clock

/**
 * Town Crier's composition root: the single place `:app` hand-wires the
 * dependency graph from `:domain` ports to `:data` implementations, via
 * manual constructor injection (epic #770 — no DI framework, no Hilt/Koin).
 * Android-touching leaves come in through the constructor as their domain
 * interfaces, so this class itself stays a pure-JVM type — which is what
 * lets [AppGraphSmokeTest] construct it in a plain JVM test.
 *
 * Empty today; each later phase of the Android parity epic adds one `val`
 * per port the UI layer consumes (see android-coding-standards skill,
 * architecture-and-modules.md).
 */
public class AppGraph(
    public val baseUrl: String,
    private val clock: Clock = Clock.systemUTC(),
) {
    // Wiring lands feature-by-feature, e.g.:
    // val watchZoneRepository: WatchZoneRepository = HttpWatchZoneRepository(apiClient)
}
