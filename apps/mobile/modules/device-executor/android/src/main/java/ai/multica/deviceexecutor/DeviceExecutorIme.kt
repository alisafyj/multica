package ai.multica.deviceexecutor

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.inputmethodservice.InputMethodService
import android.os.Build
import android.util.Base64
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.widget.TextView

/**
 * ADBKeyboard-compatible input method: the adb track's way to type anything
 * `adb shell input text` cannot (Chinese, emoji). The hub switches to this
 * IME for a lease, broadcasts the text, and switches back afterwards
 * (multica-device-mcp/src/controller/adb-backend.ts). Nothing here reaches
 * the network: the only input is a broadcast from the adb shell on this phone.
 *
 * Broadcast contract (the de facto ADBKeyboard one, so existing tooling works):
 *   ADB_INPUT_TEXT   --es msg <text>
 *   ADB_INPUT_B64    --es msg <base64 utf-8>
 *   ADB_INPUT_CHARS  --eia chars <code points>
 *   ADB_INPUT_CODE   --ei code <key code>       (down + up)
 *   ADB_EDITOR_CODE  --ei code <editor action>  (IME_ACTION_*)
 *   ADB_CLEAR_TEXT
 */
class DeviceExecutorIme : InputMethodService() {
    companion object {
        private const val ACTION_TEXT = "ADB_INPUT_TEXT"
        private const val ACTION_B64 = "ADB_INPUT_B64"
        private const val ACTION_CHARS = "ADB_INPUT_CHARS"
        private const val ACTION_CODE = "ADB_INPUT_CODE"
        private const val ACTION_EDITOR = "ADB_EDITOR_CODE"
        private const val ACTION_CLEAR = "ADB_CLEAR_TEXT"

        /** The id `ime set` and the enabled-IME setting use. */
        fun id(context: Context): String = "${context.packageName}/${DeviceExecutorIme::class.java.name}"
    }

    private val receiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context, intent: Intent) {
            val ic = currentInputConnection ?: return
            when (intent.action) {
                ACTION_TEXT -> intent.getStringExtra("msg")?.let { ic.commitText(it, 1) }
                ACTION_B64 -> intent.getStringExtra("msg")?.let { encoded ->
                    val text = try {
                        String(Base64.decode(encoded, Base64.DEFAULT), Charsets.UTF_8)
                    } catch (_: IllegalArgumentException) {
                        return
                    }
                    ic.commitText(text, 1)
                }
                ACTION_CHARS -> intent.getIntArrayExtra("chars")?.let { points ->
                    ic.commitText(String(points, 0, points.size), 1)
                }
                ACTION_CODE -> {
                    val code = intent.getIntExtra("code", -1)
                    if (code >= 0) sendDownUpKeyEvents(code)
                }
                ACTION_EDITOR -> {
                    val code = intent.getIntExtra("code", -1)
                    if (code >= 0) ic.performEditorAction(code)
                }
                ACTION_CLEAR -> {
                    ic.performContextMenuAction(android.R.id.selectAll)
                    ic.commitText("", 1)
                }
            }
        }
    }

    override fun onCreate() {
        super.onCreate()
        val filter = IntentFilter().apply {
            addAction(ACTION_TEXT)
            addAction(ACTION_B64)
            addAction(ACTION_CHARS)
            addAction(ACTION_CODE)
            addAction(ACTION_EDITOR)
            addAction(ACTION_CLEAR)
        }
        // `am broadcast` from the adb shell is an external sender, so the
        // receiver must be exported on Android 13+.
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            registerReceiver(receiver, filter, Context.RECEIVER_EXPORTED)
        } else {
            registerReceiver(receiver, filter)
        }
    }

    override fun onDestroy() {
        try {
            unregisterReceiver(receiver)
        } catch (_: IllegalArgumentException) {
            // Not registered: onCreate failed early.
        }
        super.onDestroy()
    }

    /** A one-line strip instead of keys: the tester must see which keyboard is active. */
    override fun onCreateInputView(): View {
        val padding = TypedValue.applyDimension(TypedValue.COMPLEX_UNIT_DIP, 14f, resources.displayMetrics).toInt()
        return TextView(this).apply {
            text = getString(R.string.device_executor_ime_hint)
            gravity = Gravity.CENTER
            setPadding(padding, padding, padding, padding)
            setTextColor(0xFFE4E4E7.toInt())
            setBackgroundColor(0xFF1B1F2B.toInt())
            textSize = 14f
        }
    }

    override fun onEvaluateFullscreenMode(): Boolean = false
}
