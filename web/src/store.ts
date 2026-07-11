import { create } from 'zustand'
import { api } from './api'

type AuthState = {
  authenticated: boolean
  loading: boolean
  login: (password: string) => Promise<void>
  verify: () => Promise<void>
  logout: () => Promise<void>
}

export const useAuth = create<AuthState>((set) => ({
  authenticated: false,
  loading: true,
  async login(password) {
    const data = await api<{ ok: boolean; csrf: string }>('/api/auth/login', { method: 'POST', body: { password } })
    localStorage.setItem('csrf', data.csrf)
    set({ authenticated: true })
  },
  async verify() {
    try {
      await api('/api/auth/verify')
      const csrf = await api<{ token: string }>('/api/csrf')
      localStorage.setItem('csrf', csrf.token)
      set({ authenticated: true, loading: false })
    } catch {
      localStorage.removeItem('csrf')
      set({ authenticated: false, loading: false })
    }
  },
  async logout() {
    try { await api('/api/auth/logout', { method: 'POST', body: {} }) } catch {}
    localStorage.removeItem('csrf')
    set({ authenticated: false })
  }
}))
