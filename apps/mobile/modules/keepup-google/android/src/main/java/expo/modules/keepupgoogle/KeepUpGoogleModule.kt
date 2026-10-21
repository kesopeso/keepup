package expo.modules.keepupgoogle

import android.content.Intent
import androidx.credentials.CredentialManager
import androidx.credentials.CustomCredential
import androidx.credentials.GetCredentialRequest
import androidx.credentials.exceptions.GetCredentialCancellationException
import com.google.android.libraries.identity.googleid.GetSignInWithGoogleOption
import com.google.android.libraries.identity.googleid.GoogleIdTokenCredential
import expo.modules.kotlin.modules.Module
import expo.modules.kotlin.functions.Coroutine
import expo.modules.kotlin.modules.ModuleDefinition

/** Only ID-token proof crosses the bridge, never Google profile authority. */
class KeepUpGoogleModule : Module() {
  override fun definition() = ModuleDefinition {
    Name("KeepUpGoogle")
    Events("authLink")
    Function("consumeInitialLink") {
      val activity = appContext.currentActivity
      val intent = activity?.intent
      val url = intent?.dataString
      if (url?.startsWith("https://") == true) {
        // Remove the full link from the in-process Activity Intent immediately.
        activity?.intent = Intent(intent).setData(null)
        url
      } else null
    }
    OnNewIntent { intent ->
      val url = intent.dataString
      if (url?.startsWith("https://") == true) {
        intent.data = null
        appContext.currentActivity?.intent = Intent(intent).setData(null)
        sendEvent("authLink", mapOf("url" to url))
      }
    }
    AsyncFunction("signIn") Coroutine { serverClientId: String, nonce: String ->
      val activity = appContext.currentActivity ?: error("Google sign-in requires an active screen")
      require(serverClientId.endsWith(".apps.googleusercontent.com") && nonce.matches(Regex("[A-Za-z0-9_-]{43}")))
      val option = GetSignInWithGoogleOption.Builder(serverClientId).setNonce(nonce).build()
      val request = GetCredentialRequest.Builder().addCredentialOption(option).build()
      try {
        val credential = CredentialManager.create(activity).getCredential(activity, request).credential
        check(credential is CustomCredential && credential.type == GoogleIdTokenCredential.TYPE_GOOGLE_ID_TOKEN_CREDENTIAL)
        GoogleIdTokenCredential.createFrom(credential.data).idToken
      } catch (_: GetCredentialCancellationException) {
        null
      }
    }
  }
}
