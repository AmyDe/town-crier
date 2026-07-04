package uk.towncrierapp.domain.subscriptions

/**
 * A quantitative limit that varies by subscription tier. Must remain in sync
 * with the API's `Quota` enum. Port of iOS `Quota`.
 */
public enum class Quota {
    WATCH_ZONES,
}
