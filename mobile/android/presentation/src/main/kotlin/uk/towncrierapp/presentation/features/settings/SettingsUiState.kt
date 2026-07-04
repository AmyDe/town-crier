package uk.towncrierapp.presentation.features.settings

import uk.towncrierapp.domain.auth.AuthMethod
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.presentation.designsystem.Appearance

/**
 * A completed GDPR data export's raw bytes, ready to hand to the share
 * sheet. Wraps [ByteArray] with content-based `equals`/`hashCode` so this
 * type is safe inside an otherwise-structural-equality [SettingsUiState].
 */
public class ExportedData(
    public val bytes: ByteArray,
) {
    override fun equals(other: Any?): Boolean = other is ExportedData && bytes.contentEquals(other.bytes)

    override fun hashCode(): Int = bytes.contentHashCode()
}

/** State for [SettingsScreen]/[SettingsViewModel]. */
public data class SettingsUiState(
    val isLoading: Boolean = true,
    val email: String? = null,
    val name: String? = null,
    val authMethod: AuthMethod? = null,
    val subscriptionTier: SubscriptionTier = SubscriptionTier.FREE,
    val appearance: Appearance = Appearance.System,
    val isShowingDeleteConfirmation: Boolean = false,
    val isDeletingAccount: Boolean = false,
    val deletionError: DomainError? = null,
    val isExporting: Boolean = false,
    val exportedData: ExportedData? = null,
    val exportError: DomainError? = null,
    val appVersion: String = "",
)
