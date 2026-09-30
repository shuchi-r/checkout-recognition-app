const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080').replace(/\/$/, '')

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(options.headers || {}),
    },
  })

  const data = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new Error(data.error || 'Request failed')
  }
  return data as T
}

export type User = {
  id: string
  email: string
  firstName: string
  lastName: string
}

export const api = {
  register: (payload: { email: string; firstName: string; lastName: string }) =>
    request<{ message: string; code: string; user: User }>('/api/register', {
      method: 'POST', body: JSON.stringify(payload),
    }),
  recognize: (email: string) =>
    request<{ registered: boolean }>('/api/recognize', {
      method: 'POST', body: JSON.stringify({ email }),
    }),
  login: (email: string, code: string) =>
    request<{ message: string; user: User }>('/api/login', {
      method: 'POST', body: JSON.stringify({ email, code }),
    }),
  me: () => request<User>('/api/me'),
  logout: () => request<{ message: string }>('/api/logout', { method: 'POST' }),
  checkout: (payload: { email: string; phone: string; shippingAddress: string }) =>
    request<{ message: string }>('/api/checkout', {
      method: 'POST', body: JSON.stringify(payload),
    }),
}
