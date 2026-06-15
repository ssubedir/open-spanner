import { Link, useRouter } from '@tanstack/react-router'
import { useSelector } from '@tanstack/react-store'
import { AlertCircle, Loader2, LockKeyhole, Mail, UserPlus } from 'lucide-react'
import type { FormEvent } from 'react'

import { appStore, appStoreActions } from '../app-store'
import { Button } from '../components/ui/button'

export function RegisterPage() {
  const router = useRouter()
  const error = useSelector(appStore, (state) => state.auth.registerError)
  const loading = useSelector(appStore, (state) => state.auth.loading)

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const email = String(form.get('email') || '')
    const password = String(form.get('password') || '')

    try {
      await appStoreActions.register({ email, password })
      await router.navigate({ to: '/overview' })
    } catch {
      // Store owns the visible registration error state.
    }
  }

  return (
    <main className="auth-page">
      <section className="auth-panel" aria-labelledby="auth-title">
        <div className="auth-heading">
          <div className="auth-icon"><UserPlus aria-hidden="true" /></div>
          <div>
            <h1 id="auth-title">Register</h1>
            <p>Create a dashboard account.</p>
          </div>
        </div>

        {error ? (
          <div aria-label={`Could not create account. ${error}`} className="auth-error" role="alert">
            <AlertCircle aria-hidden="true" />
            <div>
              <strong>Could not create account</strong>
              <span>{error}</span>
            </div>
          </div>
        ) : null}

        <form className="auth-form register-form" onSubmit={(event) => void submit(event)}>
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
              <input autoComplete="new-password" minLength={8} name="password" required type="password" />
            </span>
          </label>
          <Button disabled={loading} type="submit">
            {loading ? <Loader2 className="spin" aria-hidden="true" /> : <UserPlus aria-hidden="true" />}
            Create account
          </Button>
        </form>

        <div className="auth-switch">
          Already have an account? <Link to="/login">Sign in</Link>
        </div>
      </section>
    </main>
  )
}
