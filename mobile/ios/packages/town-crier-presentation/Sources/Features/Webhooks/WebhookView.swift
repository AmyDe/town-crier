import SwiftUI
import TownCrierDomain

/// The Webhooks screen, pushed from Settings.
public struct WebhookView: View {
  @StateObject private var viewModel: WebhookViewModel
  @FocusState private var isURLFocused: Bool

  public init(viewModel: WebhookViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    ScrollViewReader { proxy in
      form
        .onChange(of: viewModel.feedbackAnchor) { _, anchor in
          guard let anchor else { return }
          // The row is not in the list until the update that publishes it has run.
          Task { @MainActor in
            withAnimation { proxy.scrollTo(anchor, anchor: .center) }
          }
        }
    }
  }

  private var form: some View {
    Form {
      content
      feedbackSection
    }
    .background(Color.tcBackground)
    .scrollContentBackground(.hidden)
    .navigationTitle(WebhookCopy.title)
    #if os(iOS)
      .navigationBarTitleDisplayMode(.inline)
    #endif
    .task { await viewModel.load() }
    .onDisappear { viewModel.reset() }
    .alert(WebhookCopy.removeAlertTitle, isPresented: $viewModel.isRemoveAlertPresented) {
      Button(WebhookCopy.cancel, role: .cancel) {}
      Button(WebhookCopy.remove, role: .destructive) {
        Task { await viewModel.confirmRemove() }
      }
    } message: {
      Text(WebhookCopy.removeAlertMessage)
    }
  }

  @ViewBuilder
  private var content: some View {
    switch viewModel.state {
    case .loading:
      Section {
        ProgressView()
          .frame(maxWidth: .infinity)
          .accessibilityLabel(WebhookCopy.loading)
      }
    case .loadFailed:
      Section {
        message(WebhookCopy.errorLoad)
        textButton(WebhookCopy.tryAgain) {
          Task { await viewModel.load() }
        }
      }
    case .upsell:
      Section {
        message(WebhookCopy.upsell)
        viewPlansButton
      }
    case .empty:
      formSection
      secretSection
    case .configured(let webhook):
      if viewModel.showsEditableForm {
        formSection
        secretSection
      } else {
        pausedSection(webhook)
      }
      statusSection
      actionsSection
    }
  }

  // MARK: - Form

  private var formSection: some View {
    Section {
      message(WebhookCopy.intro)
      TextField(
        WebhookCopy.urlLabel,
        text: $viewModel.urlText,
        prompt: Text(WebhookCopy.urlPlaceholder)
      )
      .font(TCTypography.body)
      .autocorrectionDisabled()
      #if os(iOS)
        .keyboardType(.URL)
        .textInputAutocapitalization(.never)
      #endif
      .accessibilityLabel(WebhookCopy.urlLabel)
      .focused($isURLFocused)
      .submitLabel(.done)
      .onSubmit(save)
      formatField
      Button(action: save) {
        Text(WebhookCopy.save)
      }
      .buttonStyle(WebhookSaveButtonStyle())
      .disabled(!viewModel.canSave)
      .alignmentGuide(.listRowSeparatorLeading) { dimensions in dimensions[.leading] }
      docsLink
    }
  }

  @ViewBuilder
  private var formatField: some View {
    if viewModel.format == nil {
      Menu {
        ForEach(WebhookFormat.allCases, id: \.self) { format in
          Button(WebhookCopy.formatName(format)) {
            viewModel.format = format
          }
        }
      } label: {
        HStack {
          SettingsRowStyling.settingLabel(WebhookCopy.formatLabel)
          Spacer()
          SettingsRowStyling.settingValue(WebhookCopy.formatName(viewModel.format))
          Image(systemName: "chevron.up.chevron.down")
            .font(TCTypography.caption)
            .foregroundStyle(Color.tcTextSecondary)
        }
      }
    } else {
      Picker(WebhookCopy.formatLabel, selection: $viewModel.format) {
        ForEach(WebhookFormat.allCases, id: \.self) { format in
          Text(WebhookCopy.formatName(format)).tag(WebhookFormat?.some(format))
        }
      }
    }
  }

  @ViewBuilder
  private var docsLink: some View {
    if let url = URL(string: WebhookCopy.docsURLString) {
      Link(WebhookCopy.docsLink, destination: url)
        .font(TCTypography.body)
        .foregroundStyle(Color.tcAmber)
    }
  }

  private func pausedSection(_ webhook: Webhook) -> some View {
    Section {
      message(WebhookCopy.paused)
      viewPlansButton
      ExactText(value: webhook.url)
      SettingsRowStyling.settingRow(
        label: WebhookCopy.formatLabel, value: WebhookCopy.formatName(webhook.format))
    }
  }

  // MARK: - Secret

  @ViewBuilder
  private var secretSection: some View {
    if let secret = viewModel.revealedSecret {
      Section {
        ExactText(value: secret)
        textButton(WebhookCopy.copySecret) {
          viewModel.copySecret()
        }
      } header: {
        Text(WebhookCopy.secretLabel)
          .font(TCTypography.captionEmphasis)
      } footer: {
        Text(WebhookCopy.secretOnce)
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextSecondary)
      }
    }
  }

  // MARK: - Status and actions

  private var statusSection: some View {
    Section {
      HStack {
        SettingsRowStyling.settingLabel(WebhookCopy.statusLabel)
        Spacer()
        Text(viewModel.statusText ?? "")
          .font(TCTypography.bodyEmphasis)
          .foregroundStyle(statusColor)
      }
      if let reason = viewModel.switchedOffReason {
        message(reason)
      }
      if viewModel.showsDeliveryHistory {
        SettingsRowStyling.settingRow(
          label: WebhookCopy.lastSuccessLabel, value: viewModel.lastSuccessText)
        VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
          SettingsRowStyling.settingRow(
            label: WebhookCopy.lastFailureLabel, value: viewModel.lastFailureText)
          if let reason = viewModel.lastFailureReasonText {
            SettingsRowStyling.settingCaption(reason)
          }
        }
      }
    }
  }

  private var statusColor: Color {
    switch viewModel.statusTone {
    case .active: .tcStatusPermitted
    case .switchedOff: .tcStatusRejected
    case .paused, .none: .tcTextSecondary
    }
  }

  private var actionsSection: some View {
    Section {
      ForEach(viewModel.webhookActions, id: \.self) { action in
        Button(role: action == .remove ? .destructive : nil) {
          perform(action)
        } label: {
          Text(action.title)
            .font(TCTypography.body)
            .foregroundStyle(action == .remove ? Color.tcStatusRejected : Color.tcAmber)
        }
        .disabled(viewModel.isBusy)
        if action == .sendTest, let text = viewModel.testResultText {
          feedback(text, isFailure: viewModel.testResult?.delivered == false)
            .id(WebhookFeedbackAnchor.testResult)
        }
      }
    }
  }

  private func save() {
    isURLFocused = false
    Task { await viewModel.save() }
  }

  private func perform(_ action: WebhookAction) {
    isURLFocused = false
    switch action {
    case .sendTest:
      Task { await viewModel.sendTest() }
    case .rotateSecret:
      Task { await viewModel.rotateSecret() }
    case .remove:
      viewModel.requestRemove()
    }
  }

  // MARK: - Feedback

  @ViewBuilder
  private var feedbackSection: some View {
    if viewModel.isBusy || viewModel.feedbackAnchor == .error {
      Section {
        if viewModel.isBusy {
          ProgressView()
            .frame(maxWidth: .infinity)
        }
        if viewModel.feedbackAnchor == .error {
          errorRows
            .id(WebhookFeedbackAnchor.error)
        }
      }
    }
  }

  private var errorRows: some View {
    VStack(alignment: .leading, spacing: TCSpacing.small) {
      if let text = viewModel.inlineError {
        feedback(text, isFailure: true)
      }
      if let error = viewModel.error {
        feedback(error.userMessage, isFailure: true)
      }
    }
  }

  private func feedback(_ text: String, isFailure: Bool) -> some View {
    Label {
      Text(text)
        .font(TCTypography.body)
    } icon: {
      Image(systemName: isFailure ? "exclamationmark.triangle.fill" : "checkmark.circle.fill")
    }
    .foregroundStyle(isFailure ? Color.tcStatusRejected : Color.tcStatusPermitted)
  }

  // MARK: - Shared elements

  private func message(_ text: String) -> some View {
    Text(text)
      .font(TCTypography.body)
      .foregroundStyle(Color.tcTextPrimary)
  }

  private func textButton(_ title: String, action: @escaping () -> Void) -> some View {
    Button(action: action) {
      Text(title)
        .font(TCTypography.body)
        .foregroundStyle(Color.tcAmber)
    }
  }

  private var viewPlansButton: some View {
    PrimaryButton(WebhookCopy.viewPlans) {
      viewModel.viewPlans()
    }
    .alignmentGuide(.listRowSeparatorLeading) { dimensions in dimensions[.leading] }
  }
}
