import { Link, useRouter } from '@tanstack/react-router'
import { useSelector } from '@tanstack/react-store'
import { Loader2, LogIn, ShieldCheck } from 'lucide-react'
import type { FormEvent } from 'react'

import { appStore, appStoreActions } from '../app-store'
import { Button } from '../components/ui/button'

export function LoginPage() {
  const router = useRouter()
  const error = useSelector(appStore, (state) => state.auth.loginError)
  const loading = useSelector(appStore, (state) => state.auth.loading)

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
          </div>
        </div>

        {error ? <div className="error-banner">{error}</div> : null}

        <form className="auth-form" onSubmit={(event) => void submit(event)}>
          <label>
            Email
            <input autoComplete="email" name="email" placeholder="admin@example.com" required type="email" />
          </label>
          <label>
            Password
            <input autoComplete="current-password" minLength={8} name="password" required type="password" />
          </label>
          <Button disabled={loading} type="submit">
            {loading ? <Loader2 className="spin" aria-hidden="true" /> : <LogIn aria-hidden="true" />}
            Sign in
          </Button>
        </form>

        <div className="auth-switch">
          Need an account? <Link to="/register">Register</Link>
        </div>
      </section>
    </main>
  )
}
