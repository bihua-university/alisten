package io.github.bihua_university.alisten

import android.Manifest
import android.app.Activity
import android.content.pm.PackageManager
import android.content.res.Configuration
import android.os.Build
import android.os.Bundle
import android.graphics.Color
import android.view.View
import android.view.WindowInsetsController

/** The screen attaches to an engine retained by the playback service. */
class AlistenActivity : Activity() {
    override fun onCreate(state: Bundle?) {
        super.onCreate(state)
        setContentView(PlaybackRuntime.attach(this))
        configureWindow()
        if (Build.VERSION.SDK_INT >= 33 &&
            checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
            requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), 1)
        }
    }

    override fun onConfigurationChanged(config: Configuration) {
        super.onConfigurationChanged(config)
        configureWindow()
    }

    override fun onResume() {
        super.onResume()
        PlaybackRuntime.foreground(this, true)
    }

    override fun onPause() {
        PlaybackRuntime.foreground(this, false)
        super.onPause()
    }

    @Deprecated("Android 9–12 compatibility; newer systems use NSH predictive back handling.")
    override fun onBackPressed() {
        if (PlaybackRuntime.view?.handleBack() != true) moveTaskToBack(true)
    }

    override fun onDestroy() {
        PlaybackRuntime.detach(this)
        super.onDestroy()
    }

    private fun configureWindow() {
        if (Build.VERSION.SDK_INT >= 30) {
            if (Build.VERSION.SDK_INT < 35) legacyDecorFits()
            window.insetsController?.setSystemBarsAppearance(0,
                WindowInsetsController.APPEARANCE_LIGHT_STATUS_BARS or WindowInsetsController.APPEARANCE_LIGHT_NAVIGATION_BARS)
        } else {
            legacyEdgeToEdge()
        }
        if (Build.VERSION.SDK_INT < 35) legacyBarColors()
    }

    // These APIs are needed only before their platform replacements exist.
    @Suppress("DEPRECATION")
    private fun legacyDecorFits() {
        window.setDecorFitsSystemWindows(false)
    }

    @Suppress("DEPRECATION")
    private fun legacyEdgeToEdge() {
        window.decorView.systemUiVisibility = View.SYSTEM_UI_FLAG_LAYOUT_STABLE or
            View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN or View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION
    }

    @Suppress("DEPRECATION")
    private fun legacyBarColors() {
        window.statusBarColor = Color.TRANSPARENT
        window.navigationBarColor = Color.TRANSPARENT
    }
}
