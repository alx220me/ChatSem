import React from 'react'

interface UserAvatarProps {
  name: string
  size?: 'sm' | 'md'
}

function hashColor(name: string): string {
  let hash = 0
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash)
  }
  const h = Math.abs(hash) % 360
  return `hsl(${h}, 55%, 45%)`
}

function initials(name: string): string {
  const parts = name.trim().split(/\s+/)
  if (parts.length >= 2) {
    return (parts[0][0] + parts[1][0]).toUpperCase()
  }
  return name.slice(0, 2).toUpperCase()
}

export function UserAvatar({ name, size = 'md' }: UserAvatarProps): React.ReactElement {
  const dim = size === 'sm' ? 28 : 36
  const fontSize = size === 'sm' ? 11 : 14

  return (
    <div
      style={{
        width: dim,
        height: dim,
        borderRadius: '50%',
        backgroundColor: hashColor(name),
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        color: '#fff',
        fontSize,
        fontWeight: 600,
        flexShrink: 0,
        userSelect: 'none',
      }}
      title={name}
    >
      {initials(name)}
    </div>
  )
}
