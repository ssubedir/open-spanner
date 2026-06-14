import { useRouter } from '@tanstack/react-router'
import { Loader2, LogIn, ShieldCheck } from 'lucide-react'
import { type FormEvent, useState } from 'react'

import { createAuthSession, setAuthUser } from '../api'
import { Button } from '../components/ui/button'
import type { LoadState } from '../types'

export function LoginPage() {
  const router = useRouter()
  const [status, setStatus] = useState<LoadState>('idle')
  const [error, setError] = useState('')

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const email = String(form.get('email') || '')
    const password = String(form.get('password') || '')

    setStatus('loading')
    setError('')
    try {
      const session = await createAuthSession({ email, password })
      setAuthUser(session.user)
      await router.navigate({ to: '/overview' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to authenticate')
      setStatus('error')
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
          <Button disabled={status === 'loading'} type="submit">
            {status === 'loading' ? <Loader2 className="spin" aria-hidden="true" /> : <LogIn aria-hidden="true" />}
            Sign in
          </Button>
        </form>
      </section>
    </main>
  )
}
