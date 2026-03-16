# Town Crier: Product Overview

## Naming Convention
-   **Product Name:** "Town Crier" (Two words, spaced, title case) - Used for all user-facing labels, documentation titles, and UI strings.
-   **Code & Filesystem:** `town-crier` (Lowercase, hyphenated) - Used for all directory names, filenames, repository names, and technical identifiers.

## Mission
Town Crier aims to empower citizens and community groups by providing transparent, real-time access to local planning data. We believe that staying informed about changes to the built environment should be effortless and accessible to everyone.

## The Problem
Local authority planning applications are often buried in difficult-to-navigate web portals or published in obscure public notices. Residents frequently only learn about significant local developments after the consultation period has closed, missing their opportunity to provide feedback.

## The Solution
Town Crier is a mobile-first application that monitors local authority planning registers and proactively notifies users of new applications in their areas of interest.

## Core Features
1.  **Watch Zones:** Users define areas of interest by entering a postcode and radius, receiving alerts for planning applications within that zone.
2.  **Push Notifications:** Instant alerts are sent to the user's iOS device as soon as a new planning application is detected in a followed authority.
3.  **Application Monitoring:** A central feed of all recent applications across followed authorities, with the ability to filter by status or date.
4.  **Detail Deep-Dive:** View key details of an application, including descriptions, locations, and direct links to the official council portal for formal comments.

## Target Audience
-   **Local Residents:** Who want to know what is being built in their neighborhood.
-   **Community Groups:** Coordinating responses to large-scale developments.
-   **Property Professionals:** Tracking market activity and competitor applications.

## High-Level Architecture
-   **Mobile:** Native iOS app (Swift) for a high-performance, notification-centric user experience.
-   **API:** .NET 10 backend running on Azure Container Apps, optimized for low-cost, serverless execution.
-   **Data:** Azure Cosmos DB (Serverless) for storing user preferences and cached planning metadata.
-   **Ingestion:** Polling-based ingestion from [PlanIt](https://www.planit.org.uk), with a background service querying for new and updated planning applications on a configurable interval (default: 15 minutes). See [ADR 0006](adr/0006-planit-primary-data-provider.md).
