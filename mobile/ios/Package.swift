// swift-tools-version: 6.1

import PackageDescription

// SPM requires this exact top-level constant name.
// swiftlint:disable:next prefixed_toplevel_constant
let package = Package(
  name: "TownCrier",
  platforms: [
    .iOS(.v17),
    .macOS(.v14),
  ],
  products: [
    .library(name: "TownCrierDomain", targets: ["TownCrierDomain"]),
    .library(name: "TownCrierData", targets: ["TownCrierData"]),
    .library(name: "TownCrierPresentation", targets: ["TownCrierPresentation"]),
  ],
  dependencies: [
    .package(url: "https://github.com/auth0/Auth0.swift.git", from: "2.0.0")
  ],
  targets: [
    .target(
      name: "TownCrierDomain",
      path: "packages/town-crier-domain/Sources"
    ),
    .target(
      name: "TownCrierData",
      dependencies: [
        "TownCrierDomain",
        .product(name: "Auth0", package: "Auth0.swift"),
      ],
      path: "packages/town-crier-data/Sources"
    ),
    .target(
      name: "TownCrierPresentation",
      dependencies: ["TownCrierDomain"],
      path: "packages/town-crier-presentation/Sources",
      resources: [
        .process("Resources/legal"),
        .process("DesignSystem/Resources/Fonts"),
      ]
    ),
    .testTarget(
      name: "TownCrierTests",
      dependencies: [
        "TownCrierDomain",
        "TownCrierData",
        "TownCrierPresentation",
      ],
      path: "town-crier-tests/Sources"
    ),
  ]
)
