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
  const onEdit = vi.fn()

  beforeEach(() => {
    onReply.mockClear()
    onEdit.mockClear()
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

  // --- Edit tests ---

  it('shows edit button for own messages', async () => {
    const msg = makeMsg({ userId: 'user-1', seq: 1 })
    render(
      <MessageList
        messages={[msg]}
        loading={false}
        currentUserId="user-1"
        onReply={onReply}
        onEdit={onEdit}
      />,
    )
    const msgDiv = screen.getByText('Hello world').closest('[data-seq]')!
    await userEvent.hover(msgDiv)
    expect(screen.getByTitle('Редактировать сообщение')).toBeInTheDocument()
  })

  it('does not show edit button for other users messages', async () => {
    const msg = makeMsg({ userId: 'other-user', seq: 1 })
    render(
      <MessageList
        messages={[msg]}
        loading={false}
        currentUserId="user-1"
        onReply={onReply}
        onEdit={onEdit}
      />,
    )
    const msgDiv = screen.getByText('Hello world').closest('[data-seq]')!
    await userEvent.hover(msgDiv)
    expect(screen.queryByTitle('Редактировать сообщение')).not.toBeInTheDocument()
  })

  it('clicking edit button activates inline edit mode', async () => {
    const msg = makeMsg({ userId: 'user-1', seq: 1, text: 'Original text' })
    render(
      <MessageList
        messages={[msg]}
        loading={false}
        currentUserId="user-1"
        onReply={onReply}
        onEdit={onEdit}
      />,
    )
    const msgDiv = screen.getByText('Original text').closest('[data-seq]')!
    await userEvent.hover(msgDiv)
    await userEvent.click(screen.getByTitle('Редактировать сообщение'))
    const textarea = screen.getByRole('textbox')
    expect(textarea).toBeInTheDocument()
    expect((textarea as HTMLTextAreaElement).value).toBe('Original text')
  })

  it('pressing Enter in edit mode calls onEdit with new text', async () => {
    onEdit.mockResolvedValue(undefined)
    const msg = makeMsg({ userId: 'user-1', seq: 1, text: 'Original text' })
    render(
      <MessageList
        messages={[msg]}
        loading={false}
        currentUserId="user-1"
        onReply={onReply}
        onEdit={onEdit}
      />,
    )
    const msgDiv = screen.getByText('Original text').closest('[data-seq]')!
    await userEvent.hover(msgDiv)
    await userEvent.click(screen.getByTitle('Редактировать сообщение'))
    const textarea = screen.getByRole('textbox')
    await userEvent.clear(textarea)
    await userEvent.type(textarea, 'Updated text')
    await userEvent.keyboard('{Enter}')
    expect(onEdit).toHaveBeenCalledWith('msg-1', 'Updated text')
  })

  it('pressing Escape in edit mode cancels without calling onEdit', async () => {
    const msg = makeMsg({ userId: 'user-1', seq: 1, text: 'Original text' })
    render(
      <MessageList
        messages={[msg]}
        loading={false}
        currentUserId="user-1"
        onReply={onReply}
        onEdit={onEdit}
      />,
    )
    const msgDiv = screen.getByText('Original text').closest('[data-seq]')!
    await userEvent.hover(msgDiv)
    await userEvent.click(screen.getByTitle('Редактировать сообщение'))
    await userEvent.keyboard('{Escape}')
    expect(onEdit).not.toHaveBeenCalled()
    expect(screen.getByText('Original text')).toBeInTheDocument()
  })

  it('shows "(изм.)" label for edited messages', () => {
    const msg = makeMsg({ editedAt: new Date().toISOString() })
    render(<MessageList messages={[msg]} loading={false} onReply={onReply} />)
    expect(screen.getByText('(изм.)')).toBeInTheDocument()
  })

  it('does not show "(изм.)" label for non-edited messages', () => {
    const msg = makeMsg({ editedAt: undefined })
    render(<MessageList messages={[msg]} loading={false} onReply={onReply} />)
    expect(screen.queryByText('(изм.)')).not.toBeInTheDocument()
  })
})
