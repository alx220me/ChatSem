import { describe, it, expect, vi, beforeEach, beforeAll } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MessageList } from '../MessageList'
import type { Message } from '../../types'

beforeAll(() => {
  window.HTMLElement.prototype.scrollIntoView = vi.fn()
})

function makeMsg(overrides: Partial<Message> = {}): Message {
  return {
    id: 'msg-1',
    chatId: 'chat-1',
    userId: 'user-1',
    userName: 'Alice',
    text: 'Hello world',
    seq: 1,
    createdAt: new Date().toISOString(),
    ...overrides,
  }
}

describe('MessageList', () => {
  const onReply = vi.fn()

  beforeEach(() => {
    onReply.mockClear()
  })

  it('renders a list of messages', () => {
    const messages = [
      makeMsg({ id: 'a', text: 'First message', seq: 1 }),
      makeMsg({ id: 'b', text: 'Second message', seq: 2 }),
    ]
    render(<MessageList messages={messages} loading={false} onReply={onReply} />)
    expect(screen.getByText('First message')).toBeInTheDocument()
    expect(screen.getByText('Second message')).toBeInTheDocument()
  })

  it('shows skeleton rows while loading', () => {
    render(<MessageList messages={[]} loading={true} onReply={onReply} />)
    // Loading state renders skeleton divs, no message text
    expect(screen.queryByText('Hello world')).not.toBeInTheDocument()
  })

  it('renders reply quote block when message has replyToId', () => {
    const msg = makeMsg({
      replyToId: 'orig-id',
      replyToUserName: 'Bob',
      replyToText: 'Original message text',
      replyToSeq: 3,
    })
    render(<MessageList messages={[msg]} loading={false} onReply={onReply} />)
    expect(screen.getByText('Bob')).toBeInTheDocument()
    expect(screen.getByText('Original message text')).toBeInTheDocument()
  })

  it('truncates long reply quote text to 80 chars', () => {
    const longText = 'x'.repeat(100)
    const msg = makeMsg({
      replyToId: 'orig-id',
      replyToUserName: 'Bob',
      replyToText: longText,
      replyToSeq: 3,
    })
    render(<MessageList messages={[msg]} loading={false} onReply={onReply} />)
    const truncated = screen.getByText('x'.repeat(80) + '…')
    expect(truncated).toBeInTheDocument()
  })

  it('does not render reply quote block when replyToId is absent', () => {
    const msg = makeMsg({ replyToId: undefined })
    render(<MessageList messages={[msg]} loading={false} onReply={onReply} />)
    expect(screen.queryByTitle('Перейти к оригинальному сообщению')).not.toBeInTheDocument()
  })

  it('calls onReply when ↩ Reply button is clicked', async () => {
    const msg = makeMsg({ seq: 5 })
    render(
      <MessageList
        messages={[msg]}
        loading={false}
        currentUserId="other-user"
        onReply={onReply}
      />,
    )
    // Hover to show action buttons
    const msgDiv = screen.getByText('Hello world').closest('[data-seq]')!
    await userEvent.hover(msgDiv)
    const replyBtn = screen.getByTitle('Ответить')
    await userEvent.click(replyBtn)
    expect(onReply).toHaveBeenCalledWith(msg)
  })

  it('does not show ↩ Reply button for optimistic messages (seq=-1)', async () => {
    const msg = makeMsg({ seq: -1 })
    render(
      <MessageList
        messages={[msg]}
        loading={false}
        currentUserId="other-user"
        onReply={onReply}
      />,
    )
    const msgDiv = screen.getByText('Hello world').closest('[data-seq]')!
    await userEvent.hover(msgDiv)
    expect(screen.queryByTitle('Ответить')).not.toBeInTheDocument()
  })
})
