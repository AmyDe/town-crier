# Remove Groups/Social Feature — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove all social/groups features from the codebase — they are unfinished, non-revenue-driving, and add maintenance burden.

**Architecture:** Pure deletion across all layers (API domain/application/infrastructure/web, web frontend, infra). Each task targets one layer to keep diffs reviewable. After deletion, verify builds and tests pass.

**Tech Stack:** .NET 10, React/TypeScript, Pulumi C#

---

### Task 1: Delete API domain layer Groups

**Files:**
- Delete: `api/src/town-crier.domain/Groups/` (entire directory — Group.cs, GroupMember.cs, GroupRole.cs, GroupInvitation.cs, InvitationStatus.cs, UnauthorizedGroupOperationException.cs)

- [ ] **Step 1: Delete the Groups domain directory**

```bash
rm -rf api/src/town-crier.domain/Groups/
```

- [ ] **Step 2: Verify domain project builds**

Run: `dotnet build api/src/town-crier.domain/`
Expected: Build succeeds (no other domain code references Groups)

- [ ] **Step 3: Commit**

```bash
git add -A api/src/town-crier.domain/Groups/
git commit -m "refactor: remove Groups domain layer"
```

---

### Task 2: Delete API application layer Groups

**Files:**
- Delete: `api/src/town-crier.application/Groups/` (entire directory — all command/query handlers, results, interfaces, exceptions)
- Delete: `api/tests/town-crier.application.tests/Groups/` (entire directory — all handler tests, fakes, builders)

- [ ] **Step 1: Delete application Groups directories**

```bash
rm -rf api/src/town-crier.application/Groups/
rm -rf api/tests/town-crier.application.tests/Groups/
```

- [ ] **Step 2: Verify application projects build**

Run: `dotnet build api/src/town-crier.application/ && dotnet build api/tests/town-crier.application.tests/`
Expected: Build succeeds

- [ ] **Step 3: Run application tests**

Run: `dotnet test api/tests/town-crier.application.tests/`
Expected: All remaining tests pass

- [ ] **Step 4: Commit**

```bash
git add -A api/src/town-crier.application/Groups/ api/tests/town-crier.application.tests/Groups/
git commit -m "refactor: remove Groups application layer and tests"
```

---

### Task 3: Delete API infrastructure layer Groups

**Files:**
- Delete: `api/src/town-crier.infrastructure/Groups/` (entire directory — CosmosGroupRepository, CosmosGroupInvitationRepository, InMemory variants, documents)
- Delete: `api/tests/town-crier.infrastructure.tests/Groups/` (entire directory — repository tests, document tests)
- Modify: `api/src/town-crier.infrastructure/Cosmos/CosmosContainerNames.cs` — remove `Groups` constant
- Modify: `api/src/town-crier.infrastructure/Cosmos/CosmosJsonSerializerContext.cs` — remove Group-related serializable types and using

- [ ] **Step 1: Delete infrastructure Groups directories**

```bash
rm -rf api/src/town-crier.infrastructure/Groups/
rm -rf api/tests/town-crier.infrastructure.tests/Groups/
```

- [ ] **Step 2: Remove Groups constant from CosmosContainerNames.cs**

In `api/src/town-crier.infrastructure/Cosmos/CosmosContainerNames.cs`, remove:

```csharp
    public const string Groups = "Groups";
```

- [ ] **Step 3: Remove Group types from CosmosJsonSerializerContext.cs**

In `api/src/town-crier.infrastructure/Cosmos/CosmosJsonSerializerContext.cs`:

Remove the using:
```csharp
using TownCrier.Infrastructure.Groups;
```

Remove these attributes:
```csharp
[JsonSerializable(typeof(GroupDocument))]
[JsonSerializable(typeof(List<GroupDocument>))]
[JsonSerializable(typeof(GroupMemberDocument))]
[JsonSerializable(typeof(List<GroupMemberDocument>))]
[JsonSerializable(typeof(GroupInvitationDocument))]
[JsonSerializable(typeof(List<GroupInvitationDocument>))]
```

- [ ] **Step 4: Verify infrastructure projects build and tests pass**

Run: `dotnet build api/src/town-crier.infrastructure/ && dotnet test api/tests/town-crier.infrastructure.tests/`
Expected: All pass

- [ ] **Step 5: Commit**

```bash
git add -A api/src/town-crier.infrastructure/Groups/ api/tests/town-crier.infrastructure.tests/Groups/ api/src/town-crier.infrastructure/Cosmos/CosmosContainerNames.cs api/src/town-crier.infrastructure/Cosmos/CosmosJsonSerializerContext.cs
git commit -m "refactor: remove Groups infrastructure layer, Cosmos docs, and tests"
```

---

### Task 4: Remove Groups from API web layer (endpoints, DI, serializer)

**Files:**
- Delete: `api/src/town-crier.web/Endpoints/GroupEndpoints.cs`
- Delete: `api/src/town-crier.web/Endpoints/InvitationEndpoints.cs`
- Modify: `api/src/town-crier.web/Extensions/WebApplicationExtensions.cs:41-42` — remove `v1.MapGroupEndpoints()` and `v1.MapInvitationEndpoints()`
- Modify: `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs` — remove Group DI registrations and usings
- Modify: `api/src/town-crier.web/AppJsonSerializerContext.cs` — remove Group serializable types and using
- Modify: `api/tests/town-crier.web.tests/TestWebApplicationFactory.cs` — remove Group repository overrides and usings
- Modify: `api/tests/town-crier.web.tests/DependencyInjection/RepositoryOverrideTests.cs` — remove Group assertions and usings
- Modify: `api/tests/town-crier.web.tests/DependencyInjection/ServiceRegistrationExtensionsTests.cs` — remove Group assertions and using

- [ ] **Step 1: Delete endpoint files**

```bash
rm -f api/src/town-crier.web/Endpoints/GroupEndpoints.cs
rm -f api/src/town-crier.web/Endpoints/InvitationEndpoints.cs
```

- [ ] **Step 2: Remove endpoint mappings from WebApplicationExtensions.cs**

In `api/src/town-crier.web/Extensions/WebApplicationExtensions.cs`, remove these two lines:

```csharp
        v1.MapGroupEndpoints();
        v1.MapInvitationEndpoints();
```

- [ ] **Step 3: Remove Group DI registrations from ServiceCollectionExtensions.cs**

Remove the usings:
```csharp
using TownCrier.Application.Groups;
using TownCrier.Infrastructure.Groups;
```

In `AddInfrastructureServices`, remove:
```csharp
        services.AddSingleton<IGroupRepository, CosmosGroupRepository>();
        services.AddSingleton<IGroupInvitationRepository, CosmosGroupInvitationRepository>();
```

In `AddApplicationServices`, remove:
```csharp
        services.AddTransient<CreateGroupCommandHandler>();
        services.AddTransient<GetGroupQueryHandler>();
        services.AddTransient<GetUserGroupsQueryHandler>();
        services.AddTransient<InviteMemberCommandHandler>();
        services.AddTransient<AcceptInvitationCommandHandler>();
        services.AddTransient<RemoveGroupMemberCommandHandler>();
        services.AddTransient<DeleteGroupCommandHandler>();
```

- [ ] **Step 4: Remove Group types from AppJsonSerializerContext.cs**

Remove the using:
```csharp
using TownCrier.Application.Groups;
```

Remove these attributes:
```csharp
[JsonSerializable(typeof(CreateGroupCommand))]
[JsonSerializable(typeof(CreateGroupResult))]
[JsonSerializable(typeof(GetGroupResult))]
[JsonSerializable(typeof(GetUserGroupsResult))]
[JsonSerializable(typeof(InviteMemberCommand))]
[JsonSerializable(typeof(InviteMemberResult))]
```

- [ ] **Step 5: Remove Group references from test files**

In `api/tests/town-crier.web.tests/TestWebApplicationFactory.cs`, remove:
```csharp
using TownCrier.Application.Groups;
using TownCrier.Infrastructure.Groups;
```
and:
```csharp
            services.AddSingleton<IGroupRepository, InMemoryGroupRepository>();
            services.AddSingleton<IGroupInvitationRepository, InMemoryGroupInvitationRepository>();
```

In `api/tests/town-crier.web.tests/DependencyInjection/RepositoryOverrideTests.cs`, remove:
```csharp
using TownCrier.Application.Groups;
using TownCrier.Infrastructure.Groups;
```
and:
```csharp
        await Assert.That(provider.GetRequiredService<IGroupRepository>())
            .IsTypeOf<InMemoryGroupRepository>();

        await Assert.That(provider.GetRequiredService<IGroupInvitationRepository>())
            .IsTypeOf<InMemoryGroupInvitationRepository>();
```

In `api/tests/town-crier.web.tests/DependencyInjection/ServiceRegistrationExtensionsTests.cs`, remove:
```csharp
using TownCrier.Application.Groups;
```
and:
```csharp
        await Assert.That(provider.GetService<IGroupRepository>()).IsNotNull();
        await Assert.That(provider.GetService<IGroupInvitationRepository>()).IsNotNull();
```
and:
```csharp
        await Assert.That(provider.GetService<CreateGroupCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetGroupQueryHandler>()).IsNotNull();
```

- [ ] **Step 6: Verify full API builds and all tests pass**

Run: `dotnet build api/ && dotnet test api/`
Expected: All pass

- [ ] **Step 7: Commit**

```bash
git add -A api/src/town-crier.web/Endpoints/GroupEndpoints.cs api/src/town-crier.web/Endpoints/InvitationEndpoints.cs api/src/town-crier.web/Extensions/ api/src/town-crier.web/AppJsonSerializerContext.cs api/tests/town-crier.web.tests/
git commit -m "refactor: remove Groups endpoints, DI, serializer config, and web tests"
```

---

### Task 5: Remove Groups from web frontend

**Files:**
- Delete: `web/src/features/Groups/` (entire directory — pages, hooks, tests, fixtures, spies)
- Delete: `web/src/components/CommunityGroups/` (entire directory — landing page section)
- Delete: `web/src/api/groups.ts`
- Delete: `web/src/domain/ports/groups-repository.ts`
- Modify: `web/src/AppRoutes.tsx` — remove Groups imports and routes
- Modify: `web/src/components/Sidebar/Sidebar.tsx` — remove Groups nav item
- Modify: `web/src/domain/types.ts` — remove GroupId, InvitationId, GroupRole, InvitationStatus types and related interfaces/functions
- Modify: `web/src/features/LandingPage/LandingPage.tsx` — remove CommunityGroups import and usage
- Modify: `web/src/__tests__/routes.test.tsx` — remove `/groups` stub from test API client
- Modify: `web/src/components/Faq/Faq.tsx` — update FAQ about community groups to remove group-specific language

- [ ] **Step 1: Delete Groups feature directories and files**

```bash
rm -rf web/src/features/Groups/
rm -rf web/src/components/CommunityGroups/
rm -f web/src/api/groups.ts
rm -f web/src/domain/ports/groups-repository.ts
```

- [ ] **Step 2: Remove Groups routes from AppRoutes.tsx**

Remove these imports:
```tsx
import { ConnectedGroupsListPage } from './features/Groups/ConnectedGroupsListPage';
import { ConnectedGroupCreatePage } from './features/Groups/ConnectedGroupCreatePage';
import { ConnectedGroupDetailPage } from './features/Groups/ConnectedGroupDetailPage';
import { ConnectedAcceptInvitationPage } from './features/Groups/ConnectedAcceptInvitationPage';
```

Remove these routes:
```tsx
            <Route path="/groups" element={<ConnectedGroupsListPage />} />
            <Route path="/groups/new" element={<ConnectedGroupCreatePage />} />
            <Route path="/groups/:groupId" element={<ConnectedGroupDetailPage />} />
            <Route path="/invitations/:invitationId/accept" element={<ConnectedAcceptInvitationPage />} />
```

- [ ] **Step 3: Remove Groups from Sidebar.tsx**

In `web/src/components/Sidebar/Sidebar.tsx`, remove this item from `NAV_ITEMS`:
```tsx
  { label: 'Groups', to: '/groups' },
```

- [ ] **Step 4: Remove Group types from domain/types.ts**

Remove these branded types and factory functions:
```tsx
export type GroupId = Brand<string, "GroupId">;
export type InvitationId = Brand<string, "InvitationId">;
```
```tsx
export function asGroupId(value: string): GroupId {
  return value as GroupId;
}

export function asInvitationId(value: string): InvitationId {
  return value as InvitationId;
}
```

Remove these union types and guards:
```tsx
export type GroupRole = "Owner" | "Member";

const GROUP_ROLES: readonly string[] = ["Owner", "Member"];

export function isGroupRole(value: unknown): value is GroupRole {
  return typeof value === "string" && GROUP_ROLES.includes(value);
}

export type InvitationStatus = "Pending" | "Accepted" | "Declined";

const INVITATION_STATUSES: readonly string[] = ["Pending", "Accepted", "Declined"];

export function isInvitationStatus(value: unknown): value is InvitationStatus {
  return typeof value === "string" && INVITATION_STATUSES.includes(value);
}
```

Remove the entire `// Groups` section with these interfaces:
```tsx
export interface GroupMember { ... }
export interface GroupDetail { ... }
export interface GroupSummary { ... }
export interface GroupInvitation { ... }
```

Remove these request types:
```tsx
export interface CreateGroupRequest { ... }
export interface InviteMemberRequest { ... }
```

- [ ] **Step 5: Remove CommunityGroups from LandingPage.tsx**

In `web/src/features/LandingPage/LandingPage.tsx`, remove the import:
```tsx
import { CommunityGroups } from '../../components/CommunityGroups/CommunityGroups';
```

Remove the usage:
```tsx
        <CommunityGroups />
```

- [ ] **Step 6: Update routes test**

In `web/src/__tests__/routes.test.tsx`, remove this line from the `stubApiClient.get`:
```tsx
    if (path.includes('/groups')) return [] as unknown;
```

- [ ] **Step 7: Update FAQ**

In `web/src/components/Faq/Faq.tsx`, replace the community groups FAQ item:

Replace:
```tsx
  {
    question: 'Can community groups use Town Crier?',
    answer:
      'Absolutely. Neighbourhood forums, civic societies, and residents\u2019 associations use Town Crier to stay on top of planning activity in their area. The Pro plan supports multiple watch zones and team-friendly features.',
  },
```

With:
```tsx
  {
    question: 'Can organisations use Town Crier?',
    answer:
      'Absolutely. Neighbourhood forums, civic societies, and residents\u2019 associations use Town Crier to stay on top of planning activity in their area. The Pro plan supports multiple watch zones.',
  },
```

- [ ] **Step 8: Verify web builds and tests pass**

Run: `cd web && npx tsc --noEmit && npm run build && npx vitest run`
Expected: All pass

- [ ] **Step 9: Commit**

```bash
git add -A web/src/features/Groups/ web/src/components/CommunityGroups/ web/src/api/groups.ts web/src/domain/ports/groups-repository.ts web/src/AppRoutes.tsx web/src/components/Sidebar/Sidebar.tsx web/src/domain/types.ts web/src/features/LandingPage/LandingPage.tsx web/src/__tests__/routes.test.tsx web/src/components/Faq/Faq.tsx
git commit -m "refactor: remove Groups feature from web frontend"
```

---

### Task 6: Remove Groups Cosmos container from infrastructure

**Files:**
- Modify: `infra/EnvironmentStack.cs:139-140` — remove Groups container definition

- [ ] **Step 1: Remove Groups container from EnvironmentStack.cs**

In `infra/EnvironmentStack.cs`, remove:
```csharp
            // Groups — partitioned by ownerId
            new("Groups", "/ownerId"),
```

- [ ] **Step 2: Verify infra builds**

Run: `dotnet build infra/`
Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add infra/EnvironmentStack.cs
git commit -m "refactor: remove Groups Cosmos container from infrastructure"
```

---

### Task 7: Clean up worktree leftovers and final verification

**Files:**
- Delete: `.claude/worktrees/agent-a1d18b2e/` (contains stale copy of Groups code)

- [ ] **Step 1: Delete stale worktree**

```bash
rm -rf .claude/worktrees/agent-a1d18b2e/
```

- [ ] **Step 2: Full build and test verification**

```bash
dotnet build api/ && dotnet test api/
cd web && npx tsc --noEmit && npm run build && npx vitest run
dotnet build infra/
```

Expected: Everything passes with zero Group references remaining.

- [ ] **Step 3: Verify no Group references remain**

```bash
grep -r "Group" api/src/ api/tests/ web/src/ infra/ --include="*.cs" --include="*.ts" --include="*.tsx" | grep -iv "PropertyGroup\|ItemGroup\|MapGroup\|RouteGroup\|resourceGroup\|accessGroup\|OptGroup"
```

Expected: No results (or only false positives like HTML `optgroup`).

- [ ] **Step 4: Final commit if any cleanup needed, then push**

```bash
git push
```
