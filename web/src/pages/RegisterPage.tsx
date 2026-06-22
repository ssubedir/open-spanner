import { Link, useRouter } from '@tanstack/react-router'
import { useSelector } from '@tanstack/react-store'
import { AlertCircle, Loader2, LockKeyhole, Mail, UserPlus } from 'lucide-react'
import type { FormEvent } from 'react'

import { appStore, appStoreActions } from '../app-store'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'

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
          <Label className="auth-field">
            <span>Email</span>
            <span className="auth-input-shell">
              <Mail aria-hidden="true" />
              <Input
                autoComplete="email"
                className="h-auto border-0 bg-transparent p-0 shadow-none focus-visible:border-transparent focus-visible:ring-0"
                name="email"
                placeholder="admin@example.com"
                required
                type="email"
              />
            </span>
          </Label>
          <Label className="auth-field">
            <span>Password</span>
            <span className="auth-input-shell">
              <LockKeyhole aria-hidden="true" />
              <Input
                autoComplete="new-password"
                className="h-auto border-0 bg-transparent p-0 shadow-none focus-visible:border-transparent focus-visible:ring-0"
                minLength={8}
                name="password"
                required
                type="password"
              />
            </span>
          </Label>
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
