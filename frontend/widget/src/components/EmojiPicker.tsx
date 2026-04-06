import React, { useEffect, useRef } from 'react'

const EMOJI_CATEGORIES: { label: string; emojis: string[] }[] = [
  {
    label: 'Смайлики',
    emojis: ['😀', '😂', '😍', '🥰', '😎', '🤔', '😅', '😭', '😤', '🤣', '😊', '🙂', '😉', '😋', '😇'],
  },
  {
    label: 'Жесты',
    emojis: ['👍', '👎', '👋', '🤝', '👏', '🙌', '🤞', '✌️', '🤙', '💪', '🫶', '🤲', '🫡', '👌', '🤟'],
  },
  {
    label: 'Сердца',
    emojis: ['❤️', '🧡', '💛', '💚', '💙', '💜', '🖤', '🤍', '💔', '❤️‍🔥', '💕', '💞', '💓', '💗', '💖'],
  },
  {
    label: 'Природа',
    emojis: ['🐶', '🐱', '🐻', '🦊', '🐸', '🐧', '🌸', '🌺', '🌻', '⭐', '🌙', '☀️', '🌈', '❄️', '🔥'],
  },
  {
    label: 'Еда',
    emojis: ['🍕', '🍔', '🍟', '🌮', '🍣', '🍜', '🍦', '🎂', '☕', '🧋', '🍺', '🥂', '🍷', '🍉', '🍓'],
  },
  {
    label: 'Активности',
    emojis: ['⚽', '🏀', '🎮', '🎯', '🎸', '🎵', '🎉', '🎊', '🏆', '🥇', '💻', '📱', '📸', '🚀', '✈️'],
  },
  {
    label: 'Символы',
    emojis: ['✅', '❌', '⚠️', '💡', '🔑', '🔒', '💰', '📈', '📉', '🎁', '📌', '📎', '🔔', '💬', '📢'],
  },
]

interface EmojiPickerProps {
  onSelect: (emoji: string) => void
  onClose: () => void
}

export function EmojiPicker({ onSelect, onClose }: EmojiPickerProps): React.ReactElement {
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        console.debug('[EmojiPicker] click outside — closing')
        onClose()
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [onClose])

  function handleSelect(emoji: string) {
    console.debug('[EmojiPicker] selected:', emoji)
    onSelect(emoji)
  }

  return (
    <div
      ref={containerRef}
      style={{
        position: 'absolute',
        bottom: '100%',
        right: 0,
        marginBottom: 4,
        width: 320,
        maxHeight: 280,
        overflowY: 'auto',
        backgroundColor: '#fff',
        border: '1px solid #e5e5e5',
        borderRadius: 10,
        boxShadow: '0 4px 16px rgba(0,0,0,0.12)',
        padding: '8px',
        zIndex: 100,
      }}
    >
      {EMOJI_CATEGORIES.map((cat) => (
        <div key={cat.label} style={{ marginBottom: 6 }}>
          <div
            style={{
              fontSize: 11,
              color: '#888',
              marginBottom: 4,
              paddingLeft: 2,
              fontWeight: 600,
              textTransform: 'uppercase',
              letterSpacing: '0.04em',
            }}
          >
            {cat.label}
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 2 }}>
            {cat.emojis.map((emoji) => (
              <button
                key={emoji}
                onClick={() => handleSelect(emoji)}
                title={emoji}
                style={{
                  background: 'none',
                  border: 'none',
                  cursor: 'pointer',
                  fontSize: 20,
                  padding: '3px 4px',
                  borderRadius: 6,
                  lineHeight: 1,
                  transition: 'background 0.1s',
                }}
                onMouseEnter={(e) => {
                  ;(e.currentTarget as HTMLButtonElement).style.background = '#f0f0f0'
                }}
                onMouseLeave={(e) => {
                  ;(e.currentTarget as HTMLButtonElement).style.background = 'none'
                }}
              >
                {emoji}
              </button>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
