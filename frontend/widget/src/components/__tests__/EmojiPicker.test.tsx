import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { EmojiPicker } from '../EmojiPicker'

function makeRect(): DOMRect {
  return {
    top: 400, bottom: 438, left: 200, right: 238,
    width: 38, height: 38, x: 200, y: 400,
    toJSON: () => ({}),
  }
}

describe('EmojiPicker', () => {
  const onSelect = vi.fn()
  const onClose = vi.fn()

  beforeEach(() => {
    onSelect.mockClear()
    onClose.mockClear()
  })

  it('renders emoji categories and emoji buttons', () => {
    render(<EmojiPicker onSelect={onSelect} onClose={onClose} anchorRect={makeRect()} />)
    // Category labels are rendered
    expect(screen.getByText('Смайлики')).toBeInTheDocument()
    expect(screen.getByText('Жесты')).toBeInTheDocument()
    // Some emoji buttons are rendered
    expect(screen.getByTitle('😀')).toBeInTheDocument()
    expect(screen.getByTitle('👍')).toBeInTheDocument()
  })

  it('calls onSelect with correct emoji when clicked', async () => {
    render(<EmojiPicker onSelect={onSelect} onClose={onClose} anchorRect={makeRect()} />)
    const emojiBtn = screen.getByTitle('😀')
    await userEvent.click(emojiBtn)
    expect(onSelect).toHaveBeenCalledTimes(1)
    expect(onSelect).toHaveBeenCalledWith('😀')
  })

  it('calls onClose when clicking outside the picker', () => {
    render(
      <div>
        <EmojiPicker onSelect={onSelect} onClose={onClose} anchorRect={makeRect()} />
        <div data-testid="outside">outside</div>
      </div>
    )
    fireEvent.mouseDown(screen.getByTestId('outside'))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('does not call onClose when clicking inside the picker', async () => {
    render(<EmojiPicker onSelect={onSelect} onClose={onClose} anchorRect={makeRect()} />)
    const emojiBtn = screen.getByTitle('😂')
    fireEvent.mouseDown(emojiBtn)
    expect(onClose).not.toHaveBeenCalled()
  })
})
