package io.github.bihua_university.alisten

import android.app.Activity
import android.content.Intent
import android.content.MutableContextWrapper
import android.os.Handler
import android.os.Looper
import android.view.ViewGroup
import android.view.WindowManager
import java.lang.ref.WeakReference
import com.longbridge.nsh.NshView

/** One engine shared by the visible Activity and the foreground service. */
internal object PlaybackRuntime {
    var view: NshView? = null
        private set
    var keepingAlive = false
        private set
    private var context: MutableContextWrapper? = null
    private var screen = WeakReference<Activity>(null)
    private var screenActive = false
    private var playing = false
    private val main = Handler(Looper.getMainLooper())

    fun attach(activity: Activity): NshView {
        screen = WeakReference(activity)
        screenActive = false
        val current = view
        if (current != null) {
            context!!.baseContext = activity
            (current.parent as? ViewGroup)?.removeView(current)
            return current
        }
        val wrapper = MutableContextWrapper(activity)
        context = wrapper
        val created = NshView(wrapper)
        view = created
        created.setLimits(256, 4096, 30_000, 5_000, 100_000)
        created.handle("app.backgroundPlayback") { data, reply ->
            try {
                update(data)
                reply.resolve(null)
            } catch (error: RuntimeException) {
                reply.reject(error.toString())
            }
        }
        created.loadAssetBundle("nsh")
        return created
    }

    fun detach(activity: Activity) {
        if (screen.get() !== activity) return
        foreground(activity, false)
        screen.clear()
        (view?.parent as? ViewGroup)?.removeView(view)
        context?.baseContext = activity.applicationContext
        if (!keepingAlive) destroyIfUnused()
    }

    fun foreground(activity: Activity, active: Boolean) {
        if (screen.get() !== activity) return
        screenActive = active
        updateScreenAwake()
    }

    private fun updateScreenAwake() {
        val window = screen.get()?.window ?: return
        val flag = WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON
        val awake = screenActive && playing
        if ((window.attributes.flags and flag != 0) == awake) return
        if (awake) window.addFlags(flag) else window.clearFlags(flag)
    }

    private fun update(data: Any?) {
        val values = data as? Map<*, *> ?: return
        val app = context?.applicationContext ?: return
        playing = values["active"] == true && values["playing"] == true
        updateScreenAwake()
        if (values["active"] != true) {
            keepingAlive = false
            app.stopService(Intent(app, PlaybackService::class.java))
            main.post { destroyIfUnused() }
            return
        }
        val intent = Intent(app, PlaybackService::class.java)
            .putExtra("title", values["title"] as? String ?: "等待下一首歌曲")
            .putExtra("artist", values["artist"] as? String ?: "同频")
            .putExtra("album", values["album"] as? String ?: "")
            .putExtra("artwork", values["artwork"] as? String ?: "")
            .putExtra("durationMs", (values["durationMs"] as? Number)?.toLong() ?: 0)
            .putExtra("positionMs", (values["positionMs"] as? Number)?.toLong() ?: 0)
            .putExtra("playing", values["playing"] == true)
            .putExtra("wantsAudio", values["wantsAudio"] == true)
        if (!keepingAlive) {
            app.startForegroundService(intent)
            keepingAlive = true
        } else {
            app.startService(intent)
        }
    }

    fun command(name: String, data: Any? = null) {
        view?.send(name, data)
    }

    fun stopped() {
        keepingAlive = false
        playing = false
        updateScreenAwake()
        command("app.media.stop")
        main.postDelayed({ destroyIfUnused() }, 250)
    }

    private fun destroyIfUnused() {
        if (keepingAlive || screen.get() != null) return
        val previous = view ?: return
        view = null
        previous.destroy()
        context = null
    }
}
