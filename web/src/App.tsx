import { useEffect, useRef, useState } from 'react'

import { api, type User } from './api'

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

type RegisterState = {
  email: string
  firstName: string
  lastName: string
}

type CheckoutState = {
  email: string
  phone: string
  shippingAddress: string
}

export default function App() {
  const [showRegistration, setShowRegistration] = useState(false)

  const [register, setRegister] = useState<RegisterState>({
    email: '',
    firstName: '',
    lastName: '',
  })

  const [checkout, setCheckout] = useState<CheckoutState>({
    email: '',
    phone: '',
    shippingAddress: '',
  })

  const [user, setUser] = useState<User | null>(null)

  const [recognition, setRecognition] = useState<
    'idle' | 'checking' | 'registered' | 'unregistered'
  >('idle')

  const [loginOpen, setLoginOpen] = useState(false)

  const [registrationPopupOpen, setRegistrationPopupOpen] = useState(false)

  const [code, setCode] = useState('')
  const [codeError, setCodeError] = useState('')

  const [registerResult, setRegisterResult] = useState('')
  const [registrationCode, setRegistrationCode] = useState('')

  const [status, setStatus] = useState('')
  const [error, setError] = useState('')

  const recognitionTimer = useRef<number | undefined>(undefined)

  useEffect(() => {
    api
      .me()
      .then(setUser)
      .catch(() => undefined)

    return () => {
      if (recognitionTimer.current) {
        window.clearTimeout(recognitionTimer.current)
      }
    }
  }, [])

  useEffect(() => {
    setRecognition('idle')

    if (recognitionTimer.current) {
      window.clearTimeout(recognitionTimer.current)
    }

    if (!emailPattern.test(checkout.email)) {
      return
    }

    setRecognition('checking')

    recognitionTimer.current = window.setTimeout(async () => {
      try {
        const result = await api.recognize(checkout.email)

        setRecognition(
          result.registered ? 'registered' : 'unregistered'
        )

        if (result.registered && !user) {
          setCode('')
          setCodeError('')
          setLoginOpen(true)
        }
      } catch {
        setRecognition('idle')
      }
    }, 350)
  }, [checkout.email, user])

  async function handleRegister(event: React.FormEvent) {
    event.preventDefault()

    setRegisterResult('')
    setError('')

    try {
      const result = await api.register(register)

      setRegistrationCode(result.code)
      setRegisterResult('Registration successful.')
      setRegistrationPopupOpen(true)
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : 'Registration failed'
      )
    }
  }

  async function handleLogin(event: React.FormEvent) {
    event.preventDefault()

    setCodeError('')

    try {
      const result = await api.login(
        checkout.email,
        code
      )

      setUser(result.user)
      setLoginOpen(false)
      setCode('')
    } catch (err) {
      setCodeError(
        err instanceof Error
          ? err.message
          : 'Login failed'
      )
    }
  }

  async function handleCheckout(event: React.FormEvent) {
    event.preventDefault()

    setStatus('')
    setError('')

    try {
      await api.checkout(checkout)

      setStatus(
        'Checkout data recorded successfully.'
      )

      setCheckout((current) => ({
        ...current,
        phone: '',
        shippingAddress: '',
      }))
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : 'Could not save checkout data'
      )
    }
  }

  async function handleLogout() {
    await api.logout()
    setUser(null)
  }

  function continueToCheckout() {
    setRegistrationPopupOpen(false)
    setShowRegistration(false)

    setCheckout((current) => ({
      ...current,
      email: register.email,
    }))
  }

  return (
    <main className="page-shell">
      <section className="card">

        {/* HEADER */}
        <div className="brand-row">
          <div>
            <p className="eyebrow">
              CHECKOUT
            </p>

            <h1>
              User recognition demo
            </h1>
          </div>

          <button
            className="secondary"
            onClick={() =>
              setShowRegistration((value) => !value)
            }
          >
            {showRegistration
              ? 'Back to checkout'
              : 'Register'}
          </button>
        </div>

        {/* REGISTRATION */}
        {showRegistration ? (
          <section>
            <p className="intro">
              Register your email and name.
              A 6-digit login code will be
              generated after registration.
            </p>

            <form
              onSubmit={handleRegister}
              className="form-grid"
            >
              <label>
                Email

                <input
                  type="email"
                  value={register.email}
                  onChange={(event) =>
                    setRegister({
                      ...register,
                      email: event.target.value,
                    })
                  }
                  required
                />
              </label>

              <label>
                First name

                <input
                  value={register.firstName}
                  onChange={(event) =>
                    setRegister({
                      ...register,
                      firstName: event.target.value,
                    })
                  }
                  required
                />
              </label>

              <label>
                Last name

                <input
                  value={register.lastName}
                  onChange={(event) =>
                    setRegister({
                      ...register,
                      lastName: event.target.value,
                    })
                  }
                  required
                />
              </label>

              <button
                className="primary"
                type="submit"
              >
                Register
              </button>
            </form>

            {registerResult && (
              <div className="success">
                {registerResult}
              </div>
            )}

            {error && (
              <div className="error-banner">
                {error}
              </div>
            )}
          </section>
        ) : (

          /* CHECKOUT */
          <section>

            {/* SIGNED IN USER */}
            {user && (
              <div className="signed-in">
                <span>
                  Signed in as{' '}
                  <strong>
                    {user.firstName}{' '}
                    {user.lastName}
                  </strong>
                </span>

                <button
                  onClick={handleLogout}
                  type="button"
                >
                  Log out
                </button>
              </div>
            )}

            <p className="intro">
              Enter your email first. If it belongs
              to a registered user, recognition runs
              automatically while you continue the
              checkout form.
            </p>

            <form
              onSubmit={handleCheckout}
              className="form-grid"
            >

              {/* EMAIL */}
              <label>
                Email

                <input
                  type="email"
                  value={checkout.email}
                  onChange={(event) =>
                    setCheckout({
                      ...checkout,
                      email: event.target.value,
                    })
                  }
                  required
                />

                {recognition === 'checking' && (
                  <small className="hint">
                    Checking recognition…
                  </small>
                )}

                {recognition === 'registered' &&
                  !user && (
                    <small className="hint">
                      Registered email detected.
                    </small>
                  )}

                {recognition === 'unregistered' && (
                  <small className="hint">
                    No registered account found.
                    You can continue as a guest.
                  </small>
                )}
              </label>

              {/* PHONE */}
              <label>
                Phone number

                <input
                  type="tel"
                  value={checkout.phone}
                  onChange={(event) =>
                    setCheckout({
                      ...checkout,
                      phone: event.target.value,
                    })
                  }
                  required
                />
              </label>

              {/* ADDRESS */}
              <label>
                Shipping address

                <textarea
                  rows={4}
                  value={checkout.shippingAddress}
                  onChange={(event) =>
                    setCheckout({
                      ...checkout,
                      shippingAddress:
                        event.target.value,
                    })
                  }
                  required
                />
              </label>

              {/* SUBMIT */}
              <button
                className="primary"
                type="submit"
              >
                Submit checkout
              </button>
            </form>

            {status && (
              <div className="success">
                {status}
              </div>
            )}

            {error && (
              <div className="error-banner">
                {error}
              </div>
            )}
          </section>
        )}
      </section>

      {/* =====================================================
          REGISTRATION SUCCESS POPUP
          ===================================================== */}

      {registrationPopupOpen && (
        <div
          className="modal-backdrop"
          role="presentation"
        >
          <div
            className="modal registration-success-modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="registration-success-title"
          >

            {/* SUCCESS ICON */}
            <div className="success-icon">
              ✓
            </div>

            {/* TITLE */}
            <h2 id="registration-success-title">
              Registration Successful!
            </h2>

            {/* MESSAGE */}
            <p>
              Your account has been created
              successfully.
            </p>

            <p>
              Keep this 6-digit code safe.
              You will need it to log in later.
            </p>

            {/* CODE */}
            <div className="registration-code-popup">
              <span>
                Your Login Code
              </span>

              <strong>
                {registrationCode}
              </strong>
            </div>

            {/* CONTINUE */}
            <button
              type="button"
              className="primary continue-button"
              onClick={continueToCheckout}
            >
              Continue to Checkout
            </button>
          </div>
        </div>
      )}

      {/* =====================================================
          LOGIN POPUP
          ===================================================== */}

      {loginOpen && (
        <div
          className="modal-backdrop"
          role="presentation"
        >
          <div
            className="modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="login-title"
          >

            <h2 id="login-title">
              Welcome back
            </h2>

            <p>
              We found a registered account for{' '}
              <strong>
                {checkout.email}
              </strong>
              . Enter your 6-digit code to sign in.
            </p>

            <form onSubmit={handleLogin}>

              <label>
                Login code

                <input
                  inputMode="numeric"
                  maxLength={6}
                  value={code}
                  onChange={(event) =>
                    setCode(
                      event.target.value
                        .replace(/\D/g, '')
                        .slice(0, 6)
                    )
                  }
                  autoFocus
                  required
                />
              </label>

              {codeError && (
                <div className="error-banner">
                  {codeError}
                </div>
              )}

              <div className="modal-actions">

                <button
                  type="button"
                  className="secondary"
                  onClick={() =>
                    setLoginOpen(false)
                  }
                >
                  Skip for now
                </button>

                <button
                  className="primary"
                  type="submit"
                >
                  Log in
                </button>

              </div>
            </form>
          </div>
        </div>
      )}
    </main>
  )
}