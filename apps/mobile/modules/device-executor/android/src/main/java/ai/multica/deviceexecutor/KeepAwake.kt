package ai.multica.deviceexecutor

import android.content.Context
import android.os.PowerManager

/**
 * Keeps the display on while the phone is connected to a hub. An unattended
 * test phone that dims after 30s would hand every screenshot a lock screen.
 * SCREEN_DIM_WAKE_LOCK is deprecated but still the only way to keep another
 * app's screen on; the 8h cap means a forgotten lock still ends.
 */
object KeepAwake {
    private const val MAX_HOLD_MS = 8L * 60 * 60 * 1000
    private var lock: PowerManager.WakeLock? = null

    @Synchronized
    fun set(context: Context, on: Boolean) {
        if (on) {
            if (lock?.isHeld == true) return
            val pm = context.getSystemService(PowerManager::class.java) ?: return
            @Suppress("DEPRECATION")
            val flags = PowerManager.SCREEN_DIM_WAKE_LOCK or PowerManager.ACQUIRE_CAUSES_WAKEUP or PowerManager.ON_AFTER_RELEASE
            val acquired = pm.newWakeLock(flags, "multica:device-executor")
            acquired.acquire(MAX_HOLD_MS)
            lock = acquired
        } else {
            lock?.let { if (it.isHeld) it.release() }
            lock = null
        }
    }

    @Synchronized
    fun held(): Boolean = lock?.isHeld == true
}
