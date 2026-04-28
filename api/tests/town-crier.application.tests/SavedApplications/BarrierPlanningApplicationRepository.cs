using TownCrier.Application.PlanningApplications;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.Tests.SavedApplications;

/// <summary>
/// Test fake that blocks every GetByUidAsync call until <paramref name="expected" /> calls have
/// arrived. Lets a test prove that the handler dispatches hydration concurrently — a sequential
/// implementation would only ever have one call in-flight and would dead-lock the barrier.
/// </summary>
internal sealed class BarrierPlanningApplicationRepository : IPlanningApplicationRepository
{
    private readonly TaskCompletionSource gate = new(TaskCreationOptions.RunContinuationsAsynchronously);
    private readonly Lock concurrencyLock = new();
    private readonly int expected;
    private readonly Dictionary<string, PlanningApplication> store;
    private int currentInFlight;
    private int peakConcurrent;
    private int totalCalls;

    public BarrierPlanningApplicationRepository(int expected, IEnumerable<string> uids)
    {
        this.expected = expected;
        this.store = uids.ToDictionary(uid => uid, BuildApplication);
    }

    public int PeakConcurrentCalls
    {
        get
        {
            lock (this.concurrencyLock)
            {
                return this.peakConcurrent;
            }
        }
    }

    public int TotalCalls
    {
        get
        {
            lock (this.concurrencyLock)
            {
                return this.totalCalls;
            }
        }
    }

    public async Task<PlanningApplication?> GetByUidAsync(string uid, CancellationToken ct)
    {
        bool releaseAll;
        lock (this.concurrencyLock)
        {
            this.totalCalls++;
            this.currentInFlight++;
            if (this.currentInFlight > this.peakConcurrent)
            {
                this.peakConcurrent = this.currentInFlight;
            }

            releaseAll = this.currentInFlight >= this.expected;
        }

        if (releaseAll)
        {
            this.gate.TrySetResult();
        }

        try
        {
            await this.gate.Task.WaitAsync(ct).ConfigureAwait(false);
        }
        finally
        {
            lock (this.concurrencyLock)
            {
                this.currentInFlight--;
            }
        }

        this.store.TryGetValue(uid, out var app);
        return app;
    }

    public Task<PlanningApplication?> GetByUidAsync(string uid, string authorityCode, CancellationToken ct) =>
        throw new NotSupportedException();

    public Task UpsertAsync(PlanningApplication application, CancellationToken ct) =>
        throw new NotSupportedException();

    public Task<IReadOnlyCollection<PlanningApplication>> GetByAuthorityIdAsync(int authorityId, CancellationToken ct) =>
        throw new NotSupportedException();

    public Task<IReadOnlyCollection<PlanningApplication>> FindNearbyAsync(
        string authorityCode, double latitude, double longitude, double radiusMetres, CancellationToken ct) =>
        throw new NotSupportedException();

    private static PlanningApplication BuildApplication(string uid)
    {
        return new PlanningApplication(
            name: $"APP/{uid}",
            uid: uid,
            areaName: "TestArea",
            areaId: 1,
            address: "1 Test Street",
            postcode: "BA1 1AA",
            description: "Test application",
            appType: "Full",
            appState: "Undecided",
            appSize: null,
            startDate: new DateOnly(2026, 1, 15),
            decidedDate: null,
            consultedDate: null,
            longitude: -2.36,
            latitude: 51.38,
            url: null,
            link: null,
            lastDifferent: DateTimeOffset.UtcNow);
    }
}
