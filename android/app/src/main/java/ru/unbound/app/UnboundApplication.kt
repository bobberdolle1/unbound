package ru.unbound.app

import android.app.Application
import dagger.hilt.android.HiltAndroidApp

/**
 * Main application class. Initializes global state and configuration.
 */
class UnboundApplication : Application() {

    override fun onCreate() {
        super.onCreate()
        instance = this
    }

    companion object {
        lateinit var instance: UnboundApplication
            private set
    }
}
