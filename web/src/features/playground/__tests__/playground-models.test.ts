import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { getUserModels as getPlaygroundModels } from '../api'

afterEach(() => {
  vi.restoreAllMocks()
})

describe('playground getUserModels', () => {
  it('requests the group together with endpoint=openai', async () => {
    const get = vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: ['gpt-5.5', 'auto-chat'] },
    })

    const options = await getPlaygroundModels('subscription')

    expect(get).toHaveBeenCalledWith(
      '/api/user/models',
      expect.objectContaining({
        params: expect.objectContaining({
          group: 'subscription',
          endpoint: 'openai',
        }),
      })
    )
    expect(options.map((option) => option.value)).toEqual([
      'gpt-5.5',
      'auto-chat',
    ])
  })
})
