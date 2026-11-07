package io.github.bihua_university.alisten

import android.content.Context
import android.media.AudioAttributes
import android.media.AudioFocusRequest
import android.media.AudioManager
import android.media.AudioPlaybackConfiguration
import android.os.Handler

/** The service owns focus; NSH's Video uses mixWithOthers to avoid a second owner. */
internal class PlaybackFocus(context: Context, private val main: Handler) {
    private val audio = context.getSystemService(AudioManager::class.java)
    private var enabled = false
    private var closed = false
    private var permanentLoss = false
    private var observedPlayback = false
    private val retry = Runnable {
        if (!closed && enabled && permanentLoss && !hasPlayback()) request()
    }
    private val request = AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN)
        .setAudioAttributes(AudioAttributes.Builder()
            .setUsage(AudioAttributes.USAGE_MEDIA)
            .setContentType(AudioAttributes.CONTENT_TYPE_MUSIC).build())
        .setAcceptsDelayedFocusGain(true)
        .setWillPauseWhenDucked(true)
        .setOnAudioFocusChangeListener({ change ->
            if (!closed && enabled) {
                main.removeCallbacks(retry)
                if (change == AudioManager.AUDIOFOCUS_GAIN) {
                    permanentLoss = false
                    publish(true)
                } else {
                    permanentLoss = change == AudioManager.AUDIOFOCUS_LOSS
                    observedPlayback = hasPlayback()
                    publish(false)
                }
            }
        }, main).build()
    private val playback = object : AudioManager.AudioPlaybackCallback() {
        override fun onPlaybackConfigChanged(configs: MutableList<AudioPlaybackConfiguration>) {
            if (closed || !enabled || !permanentLoss) return
            main.removeCallbacks(retry)
            // Public configurations contain active players only (anonymized by Android).
            if (configs.isNotEmpty()) observedPlayback = true
            else if (observedPlayback) main.postDelayed(retry, 750)
        }
    }

    init { audio.registerAudioPlaybackCallback(playback, main) }

    fun begin() {
        if (closed || enabled) return
        enabled = true
        request()
    }

    private fun hasPlayback() = audio.activePlaybackConfigurations.isNotEmpty()

    private fun request() {
        val result = audio.requestAudioFocus(request)
        if (result == AudioManager.AUDIOFOCUS_REQUEST_GRANTED) {
            permanentLoss = false
            publish(true)
        } else {
            if (result == AudioManager.AUDIOFOCUS_REQUEST_FAILED) {
                permanentLoss = true
                observedPlayback = hasPlayback()
            }
            publish(false)
        }
    }

    private fun publish(available: Boolean) {
        PlaybackRuntime.command("app.audioFocus", mapOf("available" to available))
    }

    fun close() {
        closed = true
        main.removeCallbacks(retry)
        audio.unregisterAudioPlaybackCallback(playback)
        if (enabled) audio.abandonAudioFocusRequest(request)
    }
}
