import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MessageInput } from '../MessageInput'

describe('MessageInput', () => {
  const onSend = vi.fn()

  beforeEach(() => {
    onSend.mockClear()
  })

  it('renders textarea and Send button', () => {
    render(<MessageInput onSend={onSend} disabled={false} />)
    expect(screen.getByPlaceholderText('Введите сообщение...')).toBeInTheDocument()
    expect(screen.getByText('Send')).toBeInTheDocument()
  })

  it('renders emoji picker button', () => {
    render(<MessageInput onSend={onSend} disabled={false} />)
    expect(screen.getByTitle('Смайлики')).toBeInTheDocument()
  })

  it('shows emoji picker when 😊 button is clicked', async () => {
    render(<MessageInput onSend={onSend} disabled={false} />)
    const emojiBtn = screen.getByTitle('Смайлики')
    await userEvent.click(emojiBtn)
    expect(screen.getByText('Смайлики')).toBeInTheDocument() // category label inside picker
  })

  it('hides emoji picker when 😊 button is clicked again', async () => {
    render(<MessageInput onSend={onSend} disabled={false} />)
    const emojiBtn = screen.getByTitle('Смайлики')
    // Open
    await userEvent.click(emojiBtn)
    expect(screen.getByText('Жесты')).toBeInTheDocument()
    // Close
    await userEvent.click(emojiBtn)
    expect(screen.queryByText('Жесты')).not.toBeInTheDocument()
  })

  it('inserts selected emoji into textarea', async () => {
    render(<MessageInput onSend={onSend} disabled={false} />)
    const textarea = screen.getByPlaceholderText('Введите сообщение...')
    await userEvent.type(textarea, 'Привет')

    // Open picker and select emoji
    await userEvent.click(screen.getByTitle('Смайлики'))
    await userEvent.click(screen.getByTitle('😀'))

    expect((textarea as HTMLTextAreaElement).value).toContain('😀')
  })

  it('calls onSend with text when Send button is clicked', async () => {
    render(<MessageInput onSend={onSend} disabled={false} />)
    const textarea = screen.getByPlaceholderText('Введите сообщение...')
    await userEvent.type(textarea, 'Тестовое сообщение')
    await userEvent.click(screen.getByText('Send'))
    expect(onSend).toHaveBeenCalledWith('Тестовое сообщение')
  })

  it('does not call onSend when disabled', async () => {
    render(<MessageInput onSend={onSend} disabled={true} />)
    const textarea = screen.getByPlaceholderText('Введите сообщение...')
    await userEvent.type(textarea, 'test')
    await userEvent.click(screen.getByText('Send'))
    expect(onSend).not.toHaveBeenCalled()
  })
})
