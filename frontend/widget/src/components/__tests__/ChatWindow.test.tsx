import { describe, it, expect, vi, beforeAll } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { WidgetConfig } from '../../types'
import type { ApiClient } from '../../api/client'
import type { Chat } from '../../types'

// ── Module-level mocks (must be before component import) ─────────────────

vi.mock('../../hooks/useLongPoll', () => ({
  useLongPoll: vi.fn(),
}))

vi.mock('../../hooks/useOnline', () => ({
  useOnline: vi.fn().mockReturnValue(3),
}))

vi.mock('../../hooks/useChat', () => ({
  useChat: vi.fn().mockReturnValue({
    chat: {
      id: 'chat-1',
      eventId: 'evt-1',
      parentId: null,
      type: 'parent',
      externalRoomId: null,
    } as Chat,
    messages: [],
    initialHasMore: false,
    loading: false,
    error: null,
    sendMessage: vi.fn(),
  }),
}))

// Import component after mocks are set up
import { ChatWindow } from '../ChatWindow'

// ── Global stubs ──────────────────────────────────────────────────────────

beforeAll(() => {
  window.HTMLElement.prototype.scrollIntoView = vi.fn()

  class MockIntersectionObserver {
    observe = vi.fn()
    disconnect = vi.fn()
    unobserve = vi.fn()
    constructor() {}
  }
  Object.defineProperty(window, 'IntersectionObserver', {
    value: MockIntersectionObserver,
    writable: true,
  })

  window.HTMLElement.prototype.setPointerCapture = vi.fn()
  window.HTMLElement.prototype.releasePointerCapture = vi.fn()
})

// ── Helpers ───────────────────────────────────────────────────────────────

function makeApi(): ApiClient {
  return {
    getCurrentUserId: vi.fn().mockReturnValue('user-1'),
    getCurrentUserName: vi.fn().mockReturnValue('Alice'),
    getCurrentUserRole: vi.fn().mockReturnValue('user'),
    heartbeat: vi.fn().mockResolvedValue(undefined),
    leave: vi.fn().mockResolvedValue(undefined),
    leaveBeacon: vi.fn(),
    getOnlineCount: vi.fn().mockResolvedValue(3),
    getMessages: vi.fn().mockResolvedValue({ messages: [], has_more: false }),
    editMessage: vi.fn(),
    deleteMessage: vi.fn(),
    banUser: vi.fn(),
    muteUser: vi.fn(),
  } as unknown as ApiClient
}

function makeConfig(overrides: Partial<WidgetConfig> = {}): WidgetConfig {
  return {
    containerId: 'test',
    eventId: 'evt-1',
    token: 'tok',
    ...overrides,
  }
}

// ── Tests ─────────────────────────────────────────────────────────────────

describe('ChatWindow — embedded mode (default)', () => {
  it('renders without FAB when floating is not set', () => {
    render(<ChatWindow config={makeConfig()} api={makeApi()} />)
    expect(screen.queryByTestId('chat-fab')).toBeNull()
    expect(screen.queryByTestId('chat-collapse-btn')).toBeNull()
  })

  it('renders without floating window when floating is not set', () => {
    render(<ChatWindow config={makeConfig()} api={makeApi()} />)
    expect(screen.queryByTestId('chat-window-floating')).toBeNull()
  })
})

describe('ChatWindow — floating mode collapsed', () => {
  it('renders FAB when floating=true and defaultCollapsed=true', () => {
    render(
      <ChatWindow
        config={makeConfig({ floating: true, defaultCollapsed: true })}
        api={makeApi()}
      />,
    )
    expect(screen.getByTestId('chat-fab')).toBeDefined()
    expect(screen.queryByTestId('chat-window-floating')).toBeNull()
  })

  it('FAB has online-count badge when count > 0', () => {
    render(
      <ChatWindow
        config={makeConfig({ floating: true, defaultCollapsed: true })}
        api={makeApi()}
      />,
    )
    const fab = screen.getByTestId('chat-fab')
    // useOnline mock returns 3, badge should be visible
    const badge = fab.querySelector('span')
    expect(badge).not.toBeNull()
    expect(badge?.textContent).toBe('3')
  })

  it('expands to full window when FAB is clicked', async () => {
    const user = userEvent.setup()
    render(
      <ChatWindow
        config={makeConfig({ floating: true, defaultCollapsed: true })}
        api={makeApi()}
      />,
    )

    await user.click(screen.getByTestId('chat-fab'))

    expect(screen.queryByTestId('chat-fab')).toBeNull()
    expect(screen.getByTestId('chat-window-floating')).toBeDefined()
  })
})

describe('ChatWindow — floating mode expanded', () => {
  it('renders floating window when floating=true and defaultCollapsed=false', () => {
    render(
      <ChatWindow
        config={makeConfig({ floating: true, defaultCollapsed: false })}
        api={makeApi()}
      />,
    )
    expect(screen.queryByTestId('chat-fab')).toBeNull()
    expect(screen.getByTestId('chat-window-floating')).toBeDefined()
  })

  it('collapse button is present in the header', () => {
    render(
      <ChatWindow
        config={makeConfig({ floating: true, defaultCollapsed: false })}
        api={makeApi()}
      />,
    )
    expect(screen.getByTestId('chat-collapse-btn')).toBeDefined()
  })

  it('collapses to FAB when collapse button is clicked', async () => {
    const user = userEvent.setup()
    render(
      <ChatWindow
        config={makeConfig({ floating: true, defaultCollapsed: false })}
        api={makeApi()}
      />,
    )

    await user.click(screen.getByTestId('chat-collapse-btn'))

    expect(screen.getByTestId('chat-fab')).toBeDefined()
    expect(screen.queryByTestId('chat-window-floating')).toBeNull()
  })

  it('toggle: expand → collapse → expand preserves floating mode', async () => {
    const user = userEvent.setup()
    render(
      <ChatWindow
        config={makeConfig({ floating: true, defaultCollapsed: true })}
        api={makeApi()}
      />,
    )

    // FAB → expand
    await user.click(screen.getByTestId('chat-fab'))
    expect(screen.getByTestId('chat-window-floating')).toBeDefined()

    // Expanded → collapse
    await user.click(screen.getByTestId('chat-collapse-btn'))
    expect(screen.getByTestId('chat-fab')).toBeDefined()

    // FAB → expand again
    await user.click(screen.getByTestId('chat-fab'))
    expect(screen.getByTestId('chat-window-floating')).toBeDefined()
  })
})
