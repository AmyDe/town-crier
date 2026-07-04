package uk.towncrierapp.presentation.watchzones

import java.util.Locale

/**
 * Formats a watch-zone radius (metres) for display. Byte-exact port of iOS
 * `RadiusFormatter`'s branching — metric only, no miles:
 *
 * - `< 1000 m` shows as whole metres ("500 m").
 * - `>= 1000 m` shows as kilometres: one decimal place when fractional
 *   ("1.5 km"), no decimal when whole ("2 km").
 *
 * [Locale.UK] is pinned explicitly (rather than the device default, as iOS's
 * un-parameterised `String(format:)` does) purely for deterministic JVM unit
 * tests — this is a UK-only, metric-only app, so the decimal-separator
 * difference a non-UK JVM default locale could otherwise introduce has no
 * user-visible effect.
 */
public object RadiusFormatter {
    public fun format(metres: Double): String {
        if (metres >= 1_000.0) {
            val km = metres / 1_000.0
            return if (km % 1.0 == 0.0) {
                "${km.toInt()} km"
            } else {
                String.format(Locale.UK, "%.1f km", km)
            }
        }
        return "${metres.toInt()} m"
    }
}
