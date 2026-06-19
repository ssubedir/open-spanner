import { Link, useRouter } from '@tanstack/react-router'
import { useSelector } from '@tanstack/react-store'
import { AlertCircle, Loader2, LockKeyhole, LogIn, Mail, ShieldCheck } from 'lucide-react'
import type { FormEvent } from 'react'

import { appStore, appStoreActions } from '../app-store'
import { Button } from '../components/ui/button'

export function LoginPage() {
  const router = useRouter()
  const error = useSelector(appStore, (state) => state.auth.loginError)
  const loading = useSelector(appStore, (state) => state.auth.loading)
  const providers = useSelector(appStore, (state) => state.auth.providers.filter((provider) => provider.enabled))
  const oauthOrigin = encodeURIComponent(window.location.origin)

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const email = String(form.get('email') || '')
    const password = String(form.get('password') || '')

    try {
      await appStoreActions.login({ email, password })
      await router.navigate({ to: '/overview' })
    } catch {
      // Store owns the visible auth error state.
    }
  }

  return (
    <main className="auth-page">
      <section className="auth-panel" aria-labelledby="auth-title">
        <div className="auth-heading">
          <div className="auth-icon"><ShieldCheck aria-hidden="true" /></div>
          <div>
            <h1 id="auth-title">Sign in</h1>
            <p>Use your dashboard account.</p>
          </div>
        </div>

        {error ? (
          <div aria-label={`Could not sign in. ${error}`} className="auth-error" role="alert">
            <AlertCircle aria-hidden="true" />
            <div>
              <strong>Could not sign in</strong>
              <span>{error}</span>
            </div>
          </div>
        ) : null}

        <form className="auth-form login-form" onSubmit={(event) => void submit(event)}>
          <label className="auth-field">
            <span>Email</span>
            <span className="auth-input-shell">
              <Mail aria-hidden="true" />
              <input autoComplete="email" name="email" placeholder="admin@example.com" required type="email" />
            </span>
          </label>
          <label className="auth-field">
            <span>Password</span>
            <span className="auth-input-shell">
              <LockKeyhole aria-hidden="true" />
              <input autoComplete="current-password" minLength={8} name="password" required type="password" />
            </span>
          </label>
          <Button disabled={loading} type="submit">
            {loading ? <Loader2 className="spin" aria-hidden="true" /> : <LogIn aria-hidden="true" />}
            Sign in
          </Button>
        </form>

        {providers.length ? (
          <>
            <div className="auth-divider"><span>or</span></div>
            <div className="auth-oauth-list">
              {providers.map((provider) => (
                <a className="button button-outline button-default auth-oauth-button" href={`/v1/auth/oauth/${provider.id}?redirect_origin=${oauthOrigin}`} key={provider.id}>
                  <ProviderMark provider={provider.id} />
                  Sign in with {provider.name}
                </a>
              ))}
            </div>
          </>
        ) : null}

        <div className="auth-switch">
          Need an account? <Link to="/register">Register</Link>
        </div>
      </section>
    </main>
  )
}

function ProviderMark({ provider }: { provider: string }) {
  if (provider === 'github') {
    return (
      <svg className="provider-mark" aria-hidden="true" viewBox="0 0 24 24">
        <path
          clipRule="evenodd"
          d="M12 2a10 10 0 0 0-3.16 19.49c.5.09.68-.22.68-.48v-1.69c-2.78.6-3.37-1.34-3.37-1.34-.45-1.16-1.11-1.47-1.11-1.47-.91-.62.07-.61.07-.61 1 .07 1.53 1.03 1.53 1.03.9 1.52 2.34 1.08 2.91.83.09-.65.35-1.08.63-1.33-2.22-.25-4.55-1.11-4.55-4.94 0-1.09.39-1.98 1.03-2.68-.1-.25-.45-1.27.1-2.65 0 0 .84-.27 2.75 1.03A9.6 9.6 0 0 1 12 5.86c.85 0 1.7.11 2.5.33 1.9-1.3 2.74-1.03 2.74-1.03.55 1.38.2 2.4.1 2.65.64.7 1.03 1.59 1.03 2.68 0 3.84-2.34 4.69-4.57 4.94.36.31.68.92.68 1.85v2.74c0 .27.18.58.69.48A10 10 0 0 0 12 2Z"
          fill="currentColor"
          fillRule="evenodd"
        />
      </svg>
    )
  }
  return (
    <svg className="provider-mark" aria-hidden="true" viewBox="0 0 24 24">
      <path d="M21.6 12.23c0-.74-.07-1.45-.19-2.14H12v4.05h5.38a4.6 4.6 0 0 1-2 3.02v2.51h3.24c1.9-1.75 2.98-4.32 2.98-7.44Z" fill="#4285f4" />
      <path d="M12 22c2.7 0 4.98-.9 6.62-2.43l-3.24-2.51c-.9.6-2.04.95-3.38.95-2.61 0-4.82-1.76-5.61-4.13H3.05v2.59A10 10 0 0 0 12 22Z" fill="#34a853" />
      <path d="M6.39 13.88a6.01 6.01 0 0 1 0-3.76V7.53H3.05a10 10 0 0 0 0 8.94l3.34-2.59Z" fill="#fbbc05" />
      <path d="M12 5.99c1.47 0 2.79.5 3.82 1.5l2.87-2.87A9.62 9.62 0 0 0 12 2a10 10 0 0 0-8.95 5.53l3.34 2.59C7.18 7.75 9.39 5.99 12 5.99Z" fill="#ea4335" />
    </svg>
  )
}
