package uk.towncrierapp.app

import android.app.Activity
import android.app.Application
import android.os.Bundle
import uk.towncrierapp.data.auth.CurrentActivityProvider

/**
 * Tracks the currently-resumed [Activity] via `Application.
 * ActivityLifecycleCallbacks` — never held longer than the Activity is
 * actually in the foreground, never leaked. Registered once from
 * [TownCrierApplication.onCreate]; this is the one leaf `AppGraph` needs
 * that a real `Context`/Activity lifecycle can supply (auth `login()`/
 * `logout()` need an Activity to launch Custom Tabs).
 */
public class CurrentActivityTracker : Application.ActivityLifecycleCallbacks, CurrentActivityProvider {
    @Volatile
    private var resumedActivity: Activity? = null

    override fun currentActivity(): Activity? = resumedActivity

    override fun onActivityResumed(activity: Activity) {
        resumedActivity = activity
    }

    override fun onActivityPaused(activity: Activity) {
        if (resumedActivity === activity) resumedActivity = null
    }

    override fun onActivityCreated(
        activity: Activity,
        savedInstanceState: Bundle?,
    ) = Unit

    override fun onActivityStarted(activity: Activity) = Unit

    override fun onActivityStopped(activity: Activity) = Unit

    override fun onActivitySaveInstanceState(
        activity: Activity,
        outState: Bundle,
    ) = Unit

    override fun onActivityDestroyed(activity: Activity) = Unit
}
