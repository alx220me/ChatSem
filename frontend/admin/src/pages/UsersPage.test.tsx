import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { UsersPage } from './UsersPage'
import { AuthContext } from '../context/AuthContext'
import type { AdminApiClient } from '../api/adminClient'
import type { User } from '../types'

const mockUsers: User[] = [
  { id: 'u-1', externalId: 'ext-1', eventId: 'ev-1', name: 'Alice', role: 'user' },
  { id: 'u-2', externalId: 'ext-2', eventId: 'ev-1', name: 'Bob', role: 'moderator' },
]

function makeApi(overrides?: Partial<AdminApiClient>): AdminApiClient {
  return {
    listUsers: vi.fn().mockResolvedValue(mockUsers),
    updateUserRole: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  } as unknown as AdminApiClient
}

function renderPage(api: AdminApiClient) {
  return render(
    <AuthContext.Provider
      value={{ token: 'tok', eventId: 'ev-1', userName: 'Admin', api, login: vi.fn(), logout: vi.fn() }}
    >
      <MemoryRouter initialEntries={['/events/ev-1/users']}>
        <Routes>
          <Route path="/events/:eventId/users" element={<UsersPage />} />
        </Routes>
      </MemoryRouter>
    </AuthContext.Provider>,
  )
}

describe('UsersPage', () => {
  it('TestChangeRole_Updates: selecting moderator calls updateUserRole', async () => {
    const api = makeApi()
    renderPage(api)

    await waitFor(() => screen.getByText('Alice'))

    // Find Alice's role selector (first row = Alice = 'user')
    const selects = screen.getAllByRole('combobox')
    // First select is for Alice
    await userEvent.selectOptions(selects[0], 'moderator')

    await waitFor(() => {
      expect(api.updateUserRole).toHaveBeenCalledWith('u-1', 'moderator')
    })
  })
})
