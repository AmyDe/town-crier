/// The outcome of a planning decision, mirroring PlanIt's wire vocabulary.
public enum Decision: Equatable, Sendable {
  case permitted
  case conditions
  case rejected
}
