# Data Access (reference)

Read when writing a repository port or adapter, an `ApiClient`, or DTO / error mapping between the .NET API and the domain. The core (`SKILL.md`) states the repository-pattern rule; this file is the full prose and example.

## 6. Data Access

Data access is deferred for the landing page (no API calls). When the web app starts calling the .NET API, these patterns apply — mirroring the repository pattern used in .NET and iOS.

- **Repository Pattern:** Define interfaces (ports) in `domain/ports/`. Implement them in `data/repositories/` using `fetch`. Components and hooks never call `fetch` directly.
- **API Client:** A shared `ApiClient` in `data/api/` handles base URL, auth headers, and error mapping. Repository implementations use it, centralising HTTP mechanics.
- **DTO Mapping:** API response shapes (DTOs) are separate types from domain entities. The repository maps between them, keeping domain types clean and decoupled from API changes.
- **Error Mapping:** API errors (HTTP status codes, network failures) are mapped to typed `DomainError` instances at the repository boundary. Hooks and components only deal with domain errors.

**Example — Repository Implementation (Adapter):**
```typescript
// data/repositories/api-planning-application-repository.ts
export class ApiPlanningApplicationRepository implements PlanningApplicationRepository {
  constructor(private readonly api: ApiClient) {}

  async fetchApplications(authority: LocalAuthority): Promise<PlanningApplication[]> {
    const dtos = await this.api.get<PlanningApplicationDto[]>(
      `/authorities/${authority.code}/applications`
    );
    return dtos.map(mapToDomain);
  }

  async fetchApplication(id: PlanningApplicationId): Promise<PlanningApplication> {
    const dto = await this.api.get<PlanningApplicationDto>(`/applications/${id}`);
    return mapToDomain(dto);
  }
}

function mapToDomain(dto: PlanningApplicationDto): PlanningApplication {
  return {
    id: dto.id as PlanningApplicationId,
    reference: dto.reference as ApplicationReference,
    authority: { code: dto.authorityCode, name: dto.authorityName },
    status: dto.status as ApplicationStatus,
    receivedDate: new Date(dto.receivedDate),
    description: dto.description,
    address: dto.address,
  };
}
```
