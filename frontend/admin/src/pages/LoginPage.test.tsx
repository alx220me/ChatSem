import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { LoginPage } from './LoginPage'
import { AuthContext } from '../context/AuthContext'

// Helper: render LoginPage inside a minimal router + auth context
function renderLogin(loginFn: (username: string, password: string) => Promise<void>) {
  return render(
    <AuthContext.Provider
      value={{
        token: null,
        eventId: null,
        userName: null,
        api: null,
        login: loginFn,
        logout: vi.fn(),
      }}
    >
      <MemoryRouter initialEntries={['/login']}>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/events" element={<div data-testid="events-page">Events</div>} />
        </Routes>
      </MemoryRouter>
    </AuthContext.Provider>,
  )
}

describe('LoginPage', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('TestLogin_Success: redirects to /events on successful login', async () => {
    const login = vi.fn().mockResolvedValueOnce(undefined)
    renderLogin(login)

    await userEvent.type(screen.getByLabelText(/username/i), 'admin')
    await userEvent.type(screen.getByLabelText(/password/i), 'password123')
    await userEvent.click(screen.getByRole('button', { name: /sign in/i }))

    expect(login).toHaveBeenCalledWith('admin', 'password123')
    await waitFor(() => {
      expect(screen.getByTestId('events-page')).toBeInTheDocument()
    })
  })

  it('TestLogin_InvalidCredentials: shows error on 401', async () => {
    const login = vi.fn().mockRejectedValueOnce(new Error('Invalid credentials'))
    renderLogin(login)

    await userEvent.type(screen.getByLabelText(/username/i), 'admin')
    await userEvent.type(screen.getByLabelText(/password/i), 'wrong')
    await userEvent.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(screen.getByText('Invalid credentials')).toBeInTheDocument()
    })
  })
})
