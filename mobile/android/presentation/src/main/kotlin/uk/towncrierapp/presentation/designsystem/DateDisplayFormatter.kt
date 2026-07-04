package uk.towncrierapp.presentation.designsystem

import java.time.LocalDate
import java.time.format.DateTimeFormatter
import java.util.Locale

/**
 * Absolute dates ONLY — no relative/"time ago" text anywhere in the app
 * (byte-exact port of iOS `Date+TownCrier.swift`'s shared formatter). Any
 * `OffsetDateTime`/`Instant` this formats has already been reduced to a UTC
 * [LocalDate] by the data layer's mapping, so no timezone conversion happens
 * here.
 */
public object DateDisplayFormatter {
    private val formatter = DateTimeFormatter.ofPattern("d MMM yyyy", Locale.UK)

    public fun format(date: LocalDate): String = date.format(formatter)
}
