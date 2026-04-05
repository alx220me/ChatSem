import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { LoginPage } from './LoginPage'
import { AuthContext } from '../context/AuthContext'

// Helper: render LoginPage inside a minimal router + auth context
function renderLogin(loginFn: () => Promise<void>) {
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

    await userEvent.type(screen.getByLabelText(/event id/i), 'test-event-id')
    await userEvent.type(screen.getByLabelText(/api secret/i), 'secret123')
    await userEvent.type(screen.getByLabelText(/name/i), 'Admin')
    await userEvent.click(screen.getByRole('button', { name: /sign in/i }))

    expect(login).toHaveBeenCalledWith('test-event-id', 'secret123', 'Admin')
    await waitFor(() => {
      expect(screen.getByTestId('events-page')).toBeInTheDocument()
    })
  })

  it('TestLogin_InvalidSecret: shows error on 401', async () => {
    const login = vi.fn().mockRejectedValueOnce(new Error('Invalid secret'))
    renderLogin(login)

    await userEvent.type(screen.getByLabelText(/event id/i), 'test-event-id')
    await userEvent.type(screen.getByLabelText(/api secret/i), 'wrong')
    await userEvent.type(screen.getByLabelText(/name/i), 'Admin')
    await userEvent.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(screen.getByText('Invalid secret')).toBeInTheDocument()
    })
  })
})
