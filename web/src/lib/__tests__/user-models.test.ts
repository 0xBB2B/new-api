import { afterEach, describe, expect, it, vi } from 'vitest'

import { api, getUserModels } from '@/lib/api'

afterEach(() => {
  vi.restoreAllMocks()
})

describe('getUserModels', () => {
  it('sends no endpoint parameter when no format is given', async () => {
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValue({ data: { success: true, data: [] } })

    await getUserModels()

    expect(get).toHaveBeenCalledTimes(1)
    const config = get.mock.calls[0][1] as { params?: Record<string, unknown> }
    expect(config?.params?.endpoint).toBeUndefined()
  })

  it('sends the endpoint parameter when a format is given', async () => {
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValue({ data: { success: true, data: [] } })

    await getUserModels('openai')

    expect(get).toHaveBeenCalledWith(
      '/api/user/models',
      expect.objectContaining({
        params: expect.objectContaining({ endpoint: 'openai' }),
      })
    )
  })
})
