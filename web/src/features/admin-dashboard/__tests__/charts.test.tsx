import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  totalTokens,
  type DashboardBucket,
  type DashboardSnapshot,
} from '../api'
import { ModelDistribution } from '../charts'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))
afterEach(cleanup)

describe('Admin dashboard token display', () => {
  const bucket: DashboardBucket = {
    bucket: 0,
    user_id: 1,
    username: 'user',
    model_name: 'model',
    requests: 1,
    quota: 900,
    input_tokens: 60,
    output_tokens: 20,
    cache_creation_tokens: 10,
    cache_read_tokens: 30,
    incomplete_requests: 0,
    duration_seconds: 1,
    timed_requests: 1,
    updated_at: 0,
  }

  it('adds all four disjoint categories without treating quota as tokens', () => {
    expect(totalTokens(bucket)).toBe(120)
  })

  it('shows an empty state when all model token details are unavailable', () => {
    const unknown = {
      ...bucket,
      input_tokens: 0,
      output_tokens: 0,
      cache_creation_tokens: 0,
      cache_read_tokens: 0,
      incomplete_requests: 1,
    }
    const data: DashboardSnapshot = {
      started_at: 1,
      generated_at: 2,
      total: unknown,
      today: unknown,
      keys: 0,
      active_keys: 0,
      channels: 0,
      active_channels: 0,
      users: 0,
      new_users: 0,
      active_users: 0,
      trend: [unknown],
      models: [unknown],
      ranking: [],
      users_trend: [],
    }
    render(<ModelDistribution data={data} />)
    expect(screen.getByText('No data available')).toBeTruthy()
  })
})
