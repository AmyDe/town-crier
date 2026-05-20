using System.Reflection;
using TownCrier.Application.Notifications;
using TownCrier.Application.OfferCodes;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Web.Tests.Architecture;

/// <summary>
/// Structural guard for GH#395 Invariant 2: repository methods that execute
/// cross-partition Cosmos queries are tagged with the <c>CrossPartition</c>
/// suffix, and no user-facing handler (registered in the web DI container) may
/// call any such method. Tests use IL reflection — allowed in test code which
/// is NOT AOT-compiled.
/// </summary>
public sealed class CrossPartitionGuardTests
{
    // Admin handlers are explicitly excluded — owner-only ad-hoc tools (GH#395 "Out of scope").
    private static readonly HashSet<string> AdminHandlerNames = new(StringComparer.Ordinal)
    {
        "ListUsersQueryHandler",
        "GrantSubscriptionCommandHandler",
    };

    // Mirrors AddApplicationServices in ServiceCollectionExtensions.cs.
    // Must be kept in sync when new handlers are added to the web DI container.
    private static readonly HashSet<string> WebDiHandlerNames = new(StringComparer.Ordinal)
    {
        "GeocodePostcodeQueryHandler",
        "GetAuthoritiesQueryHandler",
        "GetAuthorityByIdQueryHandler",
        "GetDesignationContextQueryHandler",
        "CreateUserProfileCommandHandler",
        "GetUserProfileQueryHandler",
        "UpdateUserProfileCommandHandler",
        "ExportUserDataQueryHandler",
        "DeleteUserProfileCommandHandler",
        "UpdateZonePreferencesCommandHandler",
        "GetZonePreferencesQueryHandler",
        "RecordUserActivityCommandHandler",
        "CreateWatchZoneCommandHandler",
        "UpdateWatchZoneCommandHandler",
        "ListWatchZonesQueryHandler",
        "DeleteWatchZoneCommandHandler",
        "GetApplicationsByZoneQueryHandler",
        "RegisterDeviceTokenCommandHandler",
        "RemoveInvalidDeviceTokenCommandHandler",
        "GetApplicationByUidQueryHandler",
        "GetApplicationByAuthorityAndNameQueryHandler",
        "GetUserApplicationAuthoritiesQueryHandler",
        "SaveApplicationCommandHandler",
        "RemoveSavedApplicationCommandHandler",
        "GetSavedApplicationsQueryHandler",
        "GetNotificationStateQueryHandler",
        "MarkAllNotificationsReadCommandHandler",
        "AdvanceNotificationStateCommandHandler",
        "GetDemoAccountQueryHandler",
        "GenerateOfferCodesCommandHandler",
        "RedeemOfferCodeCommandHandler",
    };

    // Part 1: cross-partition suffix exists on the repository interfaces.
    // Each test below will FAIL (Red) until the methods are renamed, then
    // PASS (Green) once the interface signatures carry the CrossPartition suffix.
    [Test]
    public async Task IPlanningApplicationRepository_Should_ExposeGetByUidCrossPartitionAsync()
    {
        var method = typeof(IPlanningApplicationRepository).GetMethod(
            "GetByUidCrossPartitionAsync",
            [typeof(string), typeof(CancellationToken)]);

        await Assert.That(method).IsNotNull();
    }

    [Test]
    public async Task ISavedApplicationRepository_Should_ExposeGetUserIdsForApplicationCrossPartitionAsync()
    {
        var method = typeof(ISavedApplicationRepository).GetMethod(
            "GetUserIdsForApplicationCrossPartitionAsync",
            [typeof(string), typeof(int), typeof(CancellationToken)]);

        await Assert.That(method).IsNotNull();
    }

    [Test]
    public async Task IUserProfileRepository_Should_ExposeGetByEmailCrossPartitionAsync()
    {
        var method = typeof(IUserProfileRepository).GetMethod(
            "GetByEmailCrossPartitionAsync",
            [typeof(string), typeof(CancellationToken)]);

        await Assert.That(method).IsNotNull();
    }

    [Test]
    public async Task IUserProfileRepository_Should_ExposeGetAllByTierCrossPartitionAsync()
    {
        var method = typeof(IUserProfileRepository).GetMethod(
            "GetAllByTierCrossPartitionAsync",
            [typeof(SubscriptionTier), typeof(CancellationToken)]);

        await Assert.That(method).IsNotNull();
    }

    [Test]
    public async Task IUserProfileRepository_Should_ExposeGetAllByDigestDayCrossPartitionAsync()
    {
        var method = typeof(IUserProfileRepository).GetMethod(
            "GetAllByDigestDayCrossPartitionAsync",
            [typeof(DayOfWeek), typeof(CancellationToken)]);

        await Assert.That(method).IsNotNull();
    }

    [Test]
    public async Task IUserProfileRepository_Should_ExposeGetByOriginalTransactionIdCrossPartitionAsync()
    {
        var method = typeof(IUserProfileRepository).GetMethod(
            "GetByOriginalTransactionIdCrossPartitionAsync",
            [typeof(string), typeof(CancellationToken)]);

        await Assert.That(method).IsNotNull();
    }

    [Test]
    public async Task IUserProfileRepository_Should_ExposeGetDormantCrossPartitionAsync()
    {
        var method = typeof(IUserProfileRepository).GetMethod(
            "GetDormantCrossPartitionAsync",
            [typeof(DateTimeOffset), typeof(CancellationToken)]);

        await Assert.That(method).IsNotNull();
    }

    [Test]
    public async Task IUserProfileRepository_Should_ExposeListCrossPartitionAsync()
    {
        var method = typeof(IUserProfileRepository).GetMethod(
            "ListCrossPartitionAsync",
            [typeof(string), typeof(int), typeof(string), typeof(CancellationToken)]);

        await Assert.That(method).IsNotNull();
    }

    [Test]
    public async Task IWatchZoneRepository_Should_ExposeFindZonesContainingCrossPartitionAsync()
    {
        var method = typeof(IWatchZoneRepository).GetMethod(
            "FindZonesContainingCrossPartitionAsync",
            [typeof(double), typeof(double), typeof(CancellationToken)]);

        await Assert.That(method).IsNotNull();
    }

    [Test]
    public async Task IWatchZoneRepository_Should_ExposeGetDistinctAuthorityIdsCrossPartitionAsync()
    {
        var method = typeof(IWatchZoneRepository).GetMethod(
            "GetDistinctAuthorityIdsCrossPartitionAsync",
            [typeof(CancellationToken)]);

        await Assert.That(method).IsNotNull();
    }

    [Test]
    public async Task IWatchZoneRepository_Should_ExposeGetZoneCountsByAuthorityCrossPartitionAsync()
    {
        var method = typeof(IWatchZoneRepository).GetMethod(
            "GetZoneCountsByAuthorityCrossPartitionAsync",
            [typeof(CancellationToken)]);

        await Assert.That(method).IsNotNull();
    }

    [Test]
    public async Task INotificationRepository_Should_ExposeGetUserIdsWithUnsentEmailsCrossPartitionAsync()
    {
        var method = typeof(INotificationRepository).GetMethod(
            "GetUserIdsWithUnsentEmailsCrossPartitionAsync",
            [typeof(CancellationToken)]);

        await Assert.That(method).IsNotNull();
    }

    [Test]
    public async Task IOfferCodeRepository_Should_ExposeGetRedeemedByUserIdCrossPartitionAsync()
    {
        var method = typeof(IOfferCodeRepository).GetMethod(
            "GetRedeemedByUserIdCrossPartitionAsync",
            [typeof(string), typeof(CancellationToken)]);

        await Assert.That(method).IsNotNull();
    }

    // Part 2: no user-facing web handler calls a *CrossPartitionAsync method.
    // Uses IL inspection to walk bytecode of every HandleAsync on every handler
    // registered in the web DI. Fails hard if any call site resolves to a
    // method whose name contains "CrossPartition".
    [Test]
    public async Task No_WebFacingHandler_Should_CallCrossPartitionAsync_Method()
    {
        var applicationAssembly = typeof(IPlanningApplicationRepository).Assembly;

        var handlerTypes = applicationAssembly.GetTypes()
            .Where(t =>
                !t.IsAbstract
                && !t.IsInterface
                && (t.Name.EndsWith("CommandHandler", StringComparison.Ordinal)
                    || t.Name.EndsWith("QueryHandler", StringComparison.Ordinal))
                && WebDiHandlerNames.Contains(t.Name)
                && !AdminHandlerNames.Contains(t.Name))
            .ToList();

        var violations = new List<string>();

        foreach (var handlerType in handlerTypes)
        {
            // NonPublic needed to inspect async state-machine MoveNext methods —
            // the compiler lowers async lambdas into private nested types.
#pragma warning disable S3011 // Accessibility bypass is intentional for IL inspection of async state machines
            var methods = handlerType.GetMethods(
                BindingFlags.Instance
                | BindingFlags.Public
                | BindingFlags.NonPublic
                | BindingFlags.DeclaredOnly);
#pragma warning restore S3011

            foreach (var method in methods)
            {
                var body = method.GetMethodBody();
                if (body is null)
                {
                    continue;
                }

                var il = body.GetILAsByteArray();
                if (il is null)
                {
                    continue;
                }

                var crossPartitionCalls = FindCrossPartitionCallsInIl(handlerType.Module, il);
                foreach (var callName in crossPartitionCalls)
                {
                    violations.Add($"{handlerType.Name}.{method.Name} calls {callName}");
                }
            }
        }

        await Assert.That(violations).IsEmpty()
            .Because(
                "User-facing web handlers must not call *CrossPartitionAsync methods. "
                + "Background workers accept the cost; user requests do not. "
                + $"Violations: {string.Join(", ", violations)}");
    }

    private static List<string> FindCrossPartitionCallsInIl(Module module, byte[] il)
    {
        var found = new List<string>();
        var i = 0;

        while (i < il.Length)
        {
            int opcode = il[i++];

            // Two-byte opcode prefix
            if (opcode == 0xFE && i < il.Length)
            {
                opcode = (opcode << 8) | il[i++];
            }

            // call = 0x28, callvirt = 0x6F, ldftn = 0xFE06, ldvirtftn = 0xFE07
            bool isCallSite = opcode is 0x28 or 0x6F or 0xFE06 or 0xFE07;

            if (isCallSite && i + 3 < il.Length)
            {
                var token = il[i] | (il[i + 1] << 8) | (il[i + 2] << 16) | (il[i + 3] << 24);
                i += 4;

                try
                {
                    var calledMethod = module.ResolveMethod(token);
                    if (calledMethod is not null
                        && calledMethod.Name.Contains("CrossPartition", StringComparison.OrdinalIgnoreCase))
                    {
                        found.Add($"{calledMethod.DeclaringType?.Name}.{calledMethod.Name}");
                    }
                }
#pragma warning disable CA1031 // Swallow token resolution errors — generic tokens may not resolve
                catch (Exception)
#pragma warning restore CA1031
                {
                    // Generic instantiation tokens may not resolve; skip and continue.
                }
            }
            else
            {
                i += GetInlineOperandSize(opcode);
            }
        }

        return found;
    }

    // Returns inline operand byte count for each opcode per ECMA-335 Table III.1.
    // Used to advance the IL scanner past instructions that are not call sites.
    private static int GetInlineOperandSize(int opcode) => opcode switch
    {
        0x00 or 0x01 or 0x02 or 0x03 or 0x04 or 0x05 => 0, // nop/break/ldarg.0-3
        0x06 or 0x07 or 0x08 or 0x09 => 0,                  // ldloc.0-3
        0x0A or 0x0B or 0x0C or 0x0D => 0,                  // stloc.0-3
        0x0E or 0x0F or 0x10 or 0x11 or 0x12 or 0x13 => 1, // ldarg.s ldarga.s starg.s ldloc.s ldloca.s stloc.s
        >= 0x14 and <= 0x1E => 0,                            // ldnull ldc.i4.m1 ldc.i4.0-8
        0x1F => 1,                                           // ldc.i4.s
        0x20 => 4,                                           // ldc.i4
        0x21 => 8,                                           // ldc.i8
        0x22 => 4,                                           // ldc.r4
        0x23 => 8,                                           // ldc.r8
        0x25 or 0x26 => 0,                                   // dup pop
        0x27 => 4,                                           // jmp
        0x29 => 4,                                           // calli
        0x2A => 0,                                           // ret
        0x2B or 0x2C or 0x2D => 1,                          // br.s brfalse.s brtrue.s
        >= 0x2E and <= 0x37 => 1,                            // short branch comparisons
        >= 0x38 and <= 0x44 => 4,                            // long branches
        >= 0x46 and <= 0x57 => 0,                            // ldind/stind
        >= 0x58 and <= 0x66 => 0,                            // arithmetic/bitwise
        >= 0x67 and <= 0x6E => 0,                            // conv.i1 conv.i2 conv.i4 conv.i8 conv.r4 conv.r8 conv.u4 conv.u8
        0x70 or 0x71 or 0x72 or 0x73 or 0x74 or 0x75 => 4, // cpobj ldobj ldstr newobj castclass isinst
        0x76 => 0,                                           // conv.r.un
        0x79 => 4,                                           // unbox
        0x7A => 0,                                           // throw
        0x7B or 0x7C or 0x7D => 4,                          // ldfld ldflda stfld
        0x7E or 0x7F or 0x80 => 4,                          // ldsfld ldsflda stsfld
        0x81 => 4,                                           // stobj
        >= 0x82 and <= 0x8B => 0,                            // conv.ovf.*
        0x8C or 0x8D => 4,                                   // box newarr
        0x8E => 0,                                           // ldlen
        0x8F => 4,                                           // ldelema
        >= 0x90 and <= 0xA2 => 0,                            // ldelem/stelem short forms
        0xA3 or 0xA4 or 0xA5 => 4,                          // ldelem stelem unbox.any
        >= 0xB3 and <= 0xBA => 0,                            // conv.ovf.*
        0xC2 => 4,                                           // refanyval
        0xC3 => 0,                                           // ckfinite
        0xC6 => 4,                                           // mkrefany
        0xD0 => 4,                                           // ldtoken
        0xD1 or 0xD2 or 0xD3 => 0,                          // conv.u2 conv.u1 conv.i
        0xD4 or 0xD5 => 0,                                   // conv.ovf.i conv.ovf.u
        >= 0xD6 and <= 0xDB => 0,                            // add/sub/mul ovf variants
        0xDC => 0,                                           // endfinally
        0xDD => 4,                                           // leave
        0xDE => 1,                                           // leave.s
        0xDF => 0,                                           // stind.i
        0xE0 => 0,                                           // conv.u
        0xFE00 => 0,                                         // arglist
        >= 0xFE01 and <= 0xFE05 => 0,                        // ceq cgt cgt.un clt clt.un
        0xFE08 or 0xFE09 or 0xFE0A => 2,                    // ldarg ldarga starg
        0xFE0B or 0xFE0C or 0xFE0D => 2,                    // ldloc ldloca stloc
        0xFE0E => 0,                                         // localloc
        0xFE10 => 0,                                         // endfilter
        0xFE11 => 1,                                         // unaligned.
        0xFE12 or 0xFE13 => 0,                              // volatile. tail.
        0xFE14 or 0xFE15 => 4,                              // initobj constrained.
        0xFE16 or 0xFE17 => 0,                              // cpblk initblk
        0xFE19 => 0,                                         // rethrow
        0xFE1B => 4,                                         // sizeof
        0xFE1C or 0xFE1D => 0,                              // refanytype readonly.
        _ => 0,
    };
}
