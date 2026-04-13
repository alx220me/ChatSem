import { describe, it, expect, vi, beforeAll, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { WidgetConfig } from '../../types'
import type { ApiClient } from '../../api/client'
import { HttpError } from '../../api/client'
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
import { useChat } from '../../hooks/useChat'

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

  it('FAB shows online count when count > 0', () => {
    render(
      <ChatWindow
        config={makeConfig({ floating: true, defaultCollapsed: true })}
        api={makeApi()}
      />,
    )
    const fab = screen.getByTestId('chat-fab')
    // useOnline mock returns 3 — the number should appear somewhere inside the FAB
    expect(fab.textContent).toContain('3')
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

// ── Default mock state (shared across 429 tests) ──────────────────────────

const defaultChatState = {
  chat: {
    id: 'chat-1',
    eventId: 'evt-1',
    parentId: null,
    type: 'parent' as const,
    externalRoomId: null,
  } as Chat,
  messages: [],
  initialHasMore: false,
  loading: false,
  error: null,
  sendMessage: vi.fn(),
}

describe('ChatWindow — 429 rate limiting', () => {
  afterEach(() => {
    vi.mocked(useChat).mockReturnValue({ ...defaultChatState, sendMessage: vi.fn() })
  })

  it('shows toast with retryAfter when sendMessage is rate limited', async () => {
    const user = userEvent.setup()
    const sendMessage = vi.fn().mockRejectedValue(
      new HttpError(429, 'Too Many Requests', 5),
    )
    vi.mocked(useChat).mockReturnValue({ ...defaultChatState, sendMessage })

    render(<ChatWindow config={makeConfig()} api={makeApi()} />)

    const textarea = screen.getByRole('textbox')
    await user.type(textarea, 'hello')
    await user.keyboard('{Enter}')

    const toastMsg = await screen.findByText(/Слишком много запросов/)
    expect(toastMsg).toBeDefined()
    expect(screen.getByText(/5 сек/)).toBeDefined()
  })

  it('removes optimistic message when send is rate limited', async () => {
    const user = userEvent.setup()
    const sendMessage = vi.fn().mockRejectedValue(
      new HttpError(429, 'Too Many Requests', 0),
    )
    vi.mocked(useChat).mockReturnValue({ ...defaultChatState, sendMessage })

    render(<ChatWindow config={makeConfig()} api={makeApi()} />)

    const textarea = screen.getByRole('textbox')
    await user.type(textarea, 'my message')
    await user.keyboard('{Enter}')

    // Wait for toast to confirm the error was handled
    await screen.findByText(/Слишком много запросов/)

    // The message should not remain in the list (optimistic removed)
    expect(screen.queryByText('my message')).toBeNull()
  })

  it('shows fallback toast without countdown when retryAfter is 0', async () => {
    const user = userEvent.setup()
    const sendMessage = vi.fn().mockRejectedValue(
      new HttpError(429, 'Too Many Requests', 0),
    )
    vi.mocked(useChat).mockReturnValue({ ...defaultChatState, sendMessage })

    render(<ChatWindow config={makeConfig()} api={makeApi()} />)

    const textarea = screen.getByRole('textbox')
    await user.type(textarea, 'hello')
    await user.keyboard('{Enter}')

    const toastMsg = await screen.findByText(/Попробуйте позже/)
    expect(toastMsg).toBeDefined()
  })
})
