import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { AdminApiClient } from './adminClient'

// Helper to build a mock Response with given status and optional headers
function mockResponse(status: number, headers: Record<string, string> = {}, body = ''): Response {
  const headerMap = new Headers(headers)
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: headerMap,
    text: () => Promise.resolve(body),
    json: () => Promise.resolve({}),
  } as unknown as Response
}

describe('AdminApiClient — on429 callback', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    vi.stubGlobal('fetch', fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    fetchMock.mockReset()
  })

  it('calls on429 with retryAfter when server returns 429', async () => {
    fetchMock.mockResolvedValue(
      mockResponse(429, { 'Retry-After': '10' }),
    )

    const on429 = vi.fn()
    const client = new AdminApiClient('', () => 'token', undefined, on429)

    await expect(client.listEvents()).rejects.toThrow(/Слишком много запросов/)
    expect(on429).toHaveBeenCalledOnce()
    expect(on429).toHaveBeenCalledWith(10)
  })

  it('calls on429 with retryAfter=0 when Retry-After header is absent', async () => {
    fetchMock.mockResolvedValue(mockResponse(429))

    const on429 = vi.fn()
    const client = new AdminApiClient('', () => 'token', undefined, on429)

    await expect(client.listEvents()).rejects.toThrow(/Попробуйте позже/)
    expect(on429).toHaveBeenCalledWith(0)
  })

  it('rejects with human-readable message including countdown when retryAfter > 0', async () => {
    fetchMock.mockResolvedValue(
      mockResponse(429, { 'Retry-After': '30' }),
    )

    const client = new AdminApiClient('', () => 'token')

    await expect(client.listEvents()).rejects.toThrow('30 сек')
  })

  it('does not call on429 when server returns 401', async () => {
    fetchMock.mockResolvedValue(mockResponse(401))

    const on429 = vi.fn()
    const onUnauthorized = vi.fn()
    const client = new AdminApiClient('', () => 'token', onUnauthorized, on429)

    await expect(client.listEvents()).rejects.toThrow()
    expect(on429).not.toHaveBeenCalled()
    expect(onUnauthorized).toHaveBeenCalledOnce()
  })
})
