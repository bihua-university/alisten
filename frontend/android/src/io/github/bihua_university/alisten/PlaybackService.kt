package io.github.bihua_university.alisten

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Intent
import android.content.pm.ServiceInfo
import android.graphics.drawable.Icon
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.media.MediaMetadata
import android.media.session.MediaSession
import android.media.session.PlaybackState
import android.net.wifi.WifiManager
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.os.Build
import android.os.IBinder
import android.os.PowerManager
import android.os.Handler
import android.os.Looper
import java.io.ByteArrayOutputStream
import java.net.HttpURLConnection
import java.net.URI
import java.util.concurrent.Executors
import java.util.concurrent.Future

/** Protects an ongoing listening session without keeping the screen awake. */
class PlaybackService : Service() {
    private var session: MediaSession? = null
    private var cpu: PowerManager.WakeLock? = null
    private var wifi: WifiManager.WifiLock? = null
    private var title = "等待下一首歌曲"
    private var artist = "同频"
    private var playing = false
    private var album = ""
    private var durationMs = 0L
    private var positionMs = 0L
    private var artworkUrl = ""
    private var artwork: Bitmap? = null
    private var artworkGeneration = 0
    private val artworkExecutor = Executors.newSingleThreadExecutor()
    private var artworkTask: Future<*>? = null
    private val main = Handler(Looper.getMainLooper())
    private var focus: PlaybackFocus? = null
    private var networkCallback: ConnectivityManager.NetworkCallback? = null
    private val connectionTick = object : Runnable {
        override fun run() {
            PlaybackRuntime.command("app.connection.tick")
            main.postDelayed(this, 5000)
        }
    }

    override fun onCreate() {
        super.onCreate()
        focus = PlaybackFocus(this, main)
        main.post(connectionTick)
        val callback = object : ConnectivityManager.NetworkCallback() {
            private var validated: Network? = null
            override fun onCapabilitiesChanged(network: Network, caps: NetworkCapabilities) {
                if (caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED) && validated != network) {
                    validated = network
                    PlaybackRuntime.command("app.network.changed")
                } else if (!caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED) && validated == network) {
                    validated = null
                }
            }
            override fun onLost(network: Network) {
                if (validated == network) validated = null
                PlaybackRuntime.command("app.connection.tick")
            }
        }
        getSystemService(ConnectivityManager::class.java).registerDefaultNetworkCallback(callback, main)
        networkCallback = callback
        getSystemService(NotificationManager::class.java).createNotificationChannel(
            NotificationChannel(CHANNEL, "听歌房后台播放", NotificationManager.IMPORTANCE_LOW))
        session = MediaSession(this, "Alisten").apply {
            setCallback(object : MediaSession.Callback() {
                override fun onSkipToNext() = PlaybackRuntime.command("app.media.next")
                override fun onStop() = stopSelf()
            })
            isActive = true
        }
        cpu = getSystemService(PowerManager::class.java)
            .newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "Alisten:Playback").apply {
                setReferenceCounted(false)
                acquire()
            }
        // Android 14 retired high-performance Wi-Fi locks. On newer systems
        // media playback uses the foreground service and CPU wake lock.
        if (Build.VERSION.SDK_INT < 34) acquireLegacyWifiLock()
    }

    @Suppress("DEPRECATION")
    private fun acquireLegacyWifiLock() {
        val manager = applicationContext.getSystemService(WIFI_SERVICE) as? WifiManager
        wifi = manager?.createWifiLock(WifiManager.WIFI_MODE_FULL_HIGH_PERF, "Alisten:Streaming")?.apply {
            setReferenceCounted(false)
            acquire()
        }
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == "stop") {
            stopSelf()
            return START_NOT_STICKY
        }
        if (intent?.action == "next") PlaybackRuntime.command("app.media.next")
        intent?.getStringExtra("title")?.let { title = it }
        intent?.getStringExtra("artist")?.let { artist = it }
        intent?.getStringExtra("album")?.let { album = it }
        if (intent?.hasExtra("durationMs") == true) durationMs = intent.getLongExtra("durationMs", 0)
        if (intent?.hasExtra("positionMs") == true) positionMs = intent.getLongExtra("positionMs", 0)
        intent?.getStringExtra("artwork")?.let { loadArtwork(it) }
        if (intent?.hasExtra("playing") == true) playing = intent.getBooleanExtra("playing", false)
        updateMetadata()
        session?.setPlaybackState(PlaybackState.Builder()
            .setActions(PlaybackState.ACTION_SKIP_TO_NEXT or PlaybackState.ACTION_STOP)
            .setState(if (playing) PlaybackState.STATE_PLAYING else PlaybackState.STATE_BUFFERING,
                positionMs, if (playing) 1f else 0f).build())
        val notification = notification()
        if (Build.VERSION.SDK_INT >= 29) startForeground(NOTIFICATION, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PLAYBACK)
        else startForeground(NOTIFICATION, notification)
        // Android 15 permits focus acquisition by an active media foreground service.
        if (intent?.getBooleanExtra("wantsAudio", false) == true) focus?.begin()
        // An OS/user stop must not resurrect a silent player or stale room.
        return START_NOT_STICKY
    }

    private fun updateMetadata() {
        session?.setMetadata(MediaMetadata.Builder()
            .putString(MediaMetadata.METADATA_KEY_TITLE, title)
            .putString(MediaMetadata.METADATA_KEY_DISPLAY_TITLE, title)
            .putString(MediaMetadata.METADATA_KEY_DISPLAY_SUBTITLE, artist)
            .putString(MediaMetadata.METADATA_KEY_DISPLAY_DESCRIPTION, album)
            .putString(MediaMetadata.METADATA_KEY_ARTIST, artist)
            .putString(MediaMetadata.METADATA_KEY_ALBUM, album)
            .putLong(MediaMetadata.METADATA_KEY_DURATION, durationMs)
            .putBitmap(MediaMetadata.METADATA_KEY_ART, artwork)
            .putBitmap(MediaMetadata.METADATA_KEY_ALBUM_ART, artwork)
            .putBitmap(MediaMetadata.METADATA_KEY_DISPLAY_ICON, artwork).build())
    }

    private fun notification(): Notification {
        val open = PendingIntent.getActivity(this, 0,
            Intent(this, AlistenActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP),
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
        session?.setSessionActivity(open)
        return Notification.Builder(this, CHANNEL)
            .setSmallIcon(android.R.drawable.ic_media_play)
            .setContentTitle(title).setContentText(artist)
            .setLargeIcon(artwork)
            .setContentIntent(open).setOngoing(true).setOnlyAlertOnce(true)
            .setCategory(Notification.CATEGORY_TRANSPORT).setVisibility(Notification.VISIBILITY_PUBLIC)
            .addAction(Notification.Action.Builder(Icon.createWithResource(this, android.R.drawable.ic_media_next), "投票切歌", action("next", 1)).build())
            .addAction(Notification.Action.Builder(Icon.createWithResource(this, android.R.drawable.ic_menu_close_clear_cancel), "停止听歌", action("stop", 2)).build())
            .setStyle(Notification.MediaStyle().setMediaSession(session!!.sessionToken).setShowActionsInCompactView(0, 1))
            .build()
    }

    private fun loadArtwork(url: String) {
        if (url == artworkUrl) return
        artworkUrl = url
        artwork = null
        val generation = ++artworkGeneration
        artworkTask?.cancel(true)
        if (url.isEmpty()) return
        artworkTask = artworkExecutor.submit {
            val bitmap = readArtwork(url)
            main.post {
                if (generation != artworkGeneration || session == null) return@post
                artwork = bitmap
                updateMetadata()
                getSystemService(NotificationManager::class.java).notify(NOTIFICATION, notification())
            }
        }
    }

    private fun readArtwork(url: String): Bitmap? {
        var connection: HttpURLConnection? = null
        return try {
            val uri = URI(url)
            if (uri.scheme != "https" && uri.scheme != "http") return null
            connection = uri.toURL().openConnection() as HttpURLConnection
            connection.connectTimeout = 5000
            connection.readTimeout = 5000
            val bytes = connection.inputStream.use { input ->
                val output = ByteArrayOutputStream()
                val buffer = ByteArray(8192)
                while (!Thread.currentThread().isInterrupted) {
                    val count = input.read(buffer)
                    if (count < 0) break
                    if (output.size() + count > 2 * 1024 * 1024) return null
                    output.write(buffer, 0, count)
                }
                output.toByteArray()
            }
            val options = BitmapFactory.Options().apply { inJustDecodeBounds = true }
            BitmapFactory.decodeByteArray(bytes, 0, bytes.size, options)
            options.inSampleSize = 1
            while (maxOf(options.outWidth, options.outHeight) / options.inSampleSize > 512) options.inSampleSize *= 2
            options.inJustDecodeBounds = false
            BitmapFactory.decodeByteArray(bytes, 0, bytes.size, options)
        } catch (_: Exception) {
            null // A missing cover must never interrupt playback.
        } finally {
            connection?.disconnect()
        }
    }

    private fun action(name: String, id: Int): PendingIntent = PendingIntent.getService(this, id,
        Intent(this, PlaybackService::class.java).setAction(name),
        PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onDestroy() {
        main.removeCallbacks(connectionTick)
        networkCallback?.let { getSystemService(ConnectivityManager::class.java).unregisterNetworkCallback(it) }
        networkCallback = null
        focus?.close()
        focus = null
        ++artworkGeneration
        artworkTask?.cancel(true)
        artworkExecutor.shutdownNow()
        session?.apply { isActive = false; release() }
        session = null
        wifi?.let { if (it.isHeld) it.release() }
        cpu?.let { if (it.isHeld) it.release() }
        stopForeground(STOP_FOREGROUND_REMOVE)
        PlaybackRuntime.stopped()
        super.onDestroy()
    }

    companion object {
        private const val CHANNEL = "alisten_playback"
        private const val NOTIFICATION = 1001
    }
}
