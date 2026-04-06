import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { EventsPage } from './EventsPage'
import { AuthContext } from '../context/AuthContext'
import type { AdminApiClient } from '../api/adminClient'
import type { Event } from '../types'

const mockEvents: Event[] = [
  { id: 'ev-1', name: 'Conference A', allowedOrigin: 'https://a.com', createdAt: '2026-01-01T00:00:00Z' },
  { id: 'ev-2', name: 'Conference B', allowedOrigin: 'https://b.com', createdAt: '2026-01-02T00:00:00Z' },
]

function makeApi(overrides?: Partial<AdminApiClient>): AdminApiClient {
  return {
    listEvents: vi.fn().mockResolvedValue(mockEvents),
    createEvent: vi.fn(),
    ...overrides,
  } as unknown as AdminApiClient
}

function renderPage(api: AdminApiClient) {
  return render(
    <AuthContext.Provider
      value={{ token: 'tok', eventId: null, userName: 'Admin', api, login: vi.fn(), logout: vi.fn() }}
    >
      <MemoryRouter>
        <EventsPage />
      </MemoryRouter>
    </AuthContext.Provider>,
  )
}

describe('EventsPage', () => {
  it('TestEventsList_Renders: shows events in table', async () => {
    renderPage(makeApi())
    await waitFor(() => {
      expect(screen.getByText('Conference A')).toBeInTheDocument()
      expect(screen.getByText('Conference B')).toBeInTheDocument()
    })
  })

  it('TestCreateEvent_Form: modal submit calls createEvent and shows generated secret', async () => {
    const generatedSecret = 'a'.repeat(64)
    const newEvent: Event & { api_secret: string } = {
      id: 'ev-3',
      name: 'New Conf',
      allowedOrigin: 'https://c.com',
      createdAt: '2026-01-03T00:00:00Z',
      api_secret: generatedSecret,
    }
    const api = makeApi({ createEvent: vi.fn().mockResolvedValueOnce(newEvent) })
    renderPage(api)

    // Wait for initial load
    await waitFor(() => screen.getByText('Conference A'))

    // Open modal
    await userEvent.click(screen.getByRole('button', { name: /create event/i }))

    // Fill form — no API Secret field anymore
    const dialog = screen.getByRole('heading', { name: /create event/i }).closest('div')!
    await userEvent.type(within(dialog).getByLabelText(/^name$/i), 'New Conf')
    await userEvent.type(within(dialog).getByLabelText(/allowed origin/i), 'https://c.com')
    await userEvent.click(within(dialog).getByRole('button', { name: /create$/i }))

    expect(api.createEvent).toHaveBeenCalledWith('New Conf', 'https://c.com')
    await waitFor(() => {
      expect(screen.getByText(generatedSecret)).toBeInTheDocument()
    })
  })
})
