import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { ModerationPage } from './ModerationPage'
import { AuthContext } from '../context/AuthContext'
import type { AdminApiClient } from '../api/adminClient'
import type { Ban, Chat } from '../types'

const mockChats: Chat[] = [
  { id: 'chat-1', eventId: 'ev-1', type: 'parent', externalRoomId: null, settings: {} },
]

const mockBan: Ban = {
  id: 'ban-1',
  userId: 'user-abc',
  eventId: 'ev-1',
  reason: 'Spam',
  createdAt: '2026-01-01T00:00:00Z',
  expiresAt: null,
}

function makeApi(overrides?: Partial<AdminApiClient>): AdminApiClient {
  return {
    listChats: vi.fn().mockResolvedValue(mockChats),
    listBans: vi.fn().mockResolvedValue([mockBan]),
    createBan: vi.fn().mockResolvedValue({ ...mockBan, id: 'ban-2', userId: 'user-xyz' }),
    deleteBan: vi.fn().mockResolvedValue(undefined),
    listMutes: vi.fn().mockResolvedValue([]),
    createMute: vi.fn(),
    deleteMute: vi.fn(),
    ...overrides,
  } as unknown as AdminApiClient
}

function renderPage(api: AdminApiClient) {
  return render(
    <AuthContext.Provider
      value={{ token: 'tok', eventId: 'ev-1', userName: 'Admin', api, login: vi.fn(), logout: vi.fn() }}
    >
      <MemoryRouter initialEntries={['/events/ev-1/moderation']}>
        <Routes>
          <Route path="/events/:eventId/moderation" element={<ModerationPage />} />
        </Routes>
      </MemoryRouter>
    </AuthContext.Provider>,
  )
}

describe('ModerationPage', () => {
  it('TestBanUser_CallsApi: form submit calls createBan with correct args', async () => {
    const api = makeApi()
    renderPage(api)

    await waitFor(() => screen.getByText('Active Bans'))

    await userEvent.type(screen.getByLabelText(/user id/i), 'user-xyz')
    await userEvent.type(screen.getByLabelText(/reason/i), 'Spam')
    await userEvent.click(screen.getByRole('button', { name: /^ban$/i }))

    await waitFor(() => {
      expect(api.createBan).toHaveBeenCalledWith('user-xyz', 'ev-1', 'Spam', undefined)
    })
  })

  it('TestUnban_Confirm: Unban button shows ConfirmDialog and calls deleteBan', async () => {
    const api = makeApi()
    renderPage(api)

    // Wait for ban to appear
    await waitFor(() => screen.getByText('user-abc'))

    // Click Unban
    await userEvent.click(screen.getByRole('button', { name: /unban/i }))

    // ConfirmDialog should be visible
    const dialog = screen.getByText(/remove ban/i).closest('div[style*="position: fixed"]')!
    expect(dialog).toBeInTheDocument()

    // Confirm — click the button inside the dialog
    const confirmButtons = screen.getAllByRole('button', { name: /^unban$/i })
    await userEvent.click(confirmButtons[confirmButtons.length - 1])

    await waitFor(() => {
      expect(api.deleteBan).toHaveBeenCalledWith('ban-1')
    })
  })
})
