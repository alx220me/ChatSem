import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MessageInput } from '../MessageInput'

describe('MessageInput', () => {
  const onSend = vi.fn()
  const onCancelReply = vi.fn()

  beforeEach(() => {
    onSend.mockClear()
    onCancelReply.mockClear()
  })

  it('renders textarea and send button', () => {
    render(<MessageInput onSend={onSend} disabled={false} onCancelReply={onCancelReply} />)
    expect(screen.getByPlaceholderText('Введите сообщение...')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Отправить' })).toBeInTheDocument()
  })

  it('renders emoji picker button', () => {
    render(<MessageInput onSend={onSend} disabled={false} onCancelReply={onCancelReply} />)
    expect(screen.getByTitle('Смайлики')).toBeInTheDocument()
  })

  it('shows emoji picker when 😊 button is clicked', async () => {
    render(<MessageInput onSend={onSend} disabled={false} onCancelReply={onCancelReply} />)
    const emojiBtn = screen.getByTitle('Смайлики')
    await userEvent.click(emojiBtn)
    expect(screen.getByText('Смайлики')).toBeInTheDocument() // category label inside picker
  })

  it('hides emoji picker when 😊 button is clicked again', async () => {
    render(<MessageInput onSend={onSend} disabled={false} onCancelReply={onCancelReply} />)
    const emojiBtn = screen.getByTitle('Смайлики')
    // Open
    await userEvent.click(emojiBtn)
    expect(screen.getByText('Жесты')).toBeInTheDocument()
    // Close
    await userEvent.click(emojiBtn)
    expect(screen.queryByText('Жесты')).not.toBeInTheDocument()
  })

  it('inserts selected emoji into textarea', async () => {
    render(<MessageInput onSend={onSend} disabled={false} onCancelReply={onCancelReply} />)
    const textarea = screen.getByPlaceholderText('Введите сообщение...')
    await userEvent.type(textarea, 'Привет')

    // Open picker and select emoji
    await userEvent.click(screen.getByTitle('Смайлики'))
    await userEvent.click(screen.getByTitle('😀'))

    expect((textarea as HTMLTextAreaElement).value).toContain('😀')
  })

  it('calls onSend with text when Send button is clicked', async () => {
    render(<MessageInput onSend={onSend} disabled={false} onCancelReply={onCancelReply} />)
    const textarea = screen.getByPlaceholderText('Введите сообщение...')
    await userEvent.type(textarea, 'Тестовое сообщение')
    await userEvent.click(screen.getByRole('button', { name: 'Отправить' }))
    expect(onSend).toHaveBeenCalledWith('Тестовое сообщение', undefined)
  })

  it('does not call onSend when disabled', async () => {
    render(<MessageInput onSend={onSend} disabled={true} onCancelReply={onCancelReply} />)
    const textarea = screen.getByPlaceholderText('Введите сообщение...')
    await userEvent.type(textarea, 'test')
    await userEvent.click(screen.getByRole('button', { name: 'Отправить' }))
    expect(onSend).not.toHaveBeenCalled()
  })

  it('shows reply banner when replyingTo is provided', () => {
    const msg = { id: 'msg-1', chatId: 'c1', userId: 'u1', userName: 'Alice', text: 'Hello!', seq: 1, createdAt: '' }
    render(<MessageInput onSend={onSend} disabled={false} replyingTo={msg} onCancelReply={onCancelReply} />)
    expect(screen.getByText(/Alice/)).toBeInTheDocument()
    expect(screen.getByText('Hello!')).toBeInTheDocument()
    expect(screen.getByTitle('Отменить ответ')).toBeInTheDocument()
  })

  it('calls onCancelReply when × button is clicked', async () => {
    const msg = { id: 'msg-1', chatId: 'c1', userId: 'u1', userName: 'Bob', text: 'Hi', seq: 2, createdAt: '' }
    render(<MessageInput onSend={onSend} disabled={false} replyingTo={msg} onCancelReply={onCancelReply} />)
    await userEvent.click(screen.getByTitle('Отменить ответ'))
    expect(onCancelReply).toHaveBeenCalledTimes(1)
  })

  it('sends with replyToId when replyingTo is set', async () => {
    const msg = { id: 'reply-id-42', chatId: 'c1', userId: 'u1', userName: 'Carol', text: 'Quoted', seq: 5, createdAt: '' }
    render(<MessageInput onSend={onSend} disabled={false} replyingTo={msg} onCancelReply={onCancelReply} />)
    const textarea = screen.getByPlaceholderText('Введите сообщение...')
    await userEvent.type(textarea, 'My reply')
    await userEvent.click(screen.getByRole('button', { name: 'Отправить' }))
    expect(onSend).toHaveBeenCalledWith('My reply', 'reply-id-42')
  })
})
