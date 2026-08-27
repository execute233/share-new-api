import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'sonner'
import { beforeEach, describe, expect, test, vi } from 'vitest'

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

const { api } = await import('@/lib/api')
const { Proxies } = await import('../index')

type ApiMethod = (url: string, data?: unknown) => Promise<{ data: unknown }>
type MockableApi = { get: ApiMethod; post: ApiMethod }

const apiClient = api as unknown as MockableApi

const proxySummary = {
  id: 1,
  name: 'p1',
  protocol: 'http',
  host: 'example.com',
  port: 8080,
  status: 'active',
  credential_configured: false,
  credential_decrypt_failed: false,
  bound_channel_count: 0,
}

const summaryWithItems = {
  ...proxySummary,
  quality_status: 'warn',
  quality_score: 70,
  quality_grade: 'B',
  quality_summary: '通过 1 项，告警 1 项，失败 2 项，挑战 1 项',
  ip_address: '203.0.113.8',
  latency_ms: 123,
  last_checked_at: 1787829618,
  quality_items: [
    {
      target: 'openai',
      url: 'https://api.openai.com/v1/models',
      status: 'pass',
      http_status: 401,
      latency_ms: 300,
      message: '目标可达',
    },
  ],
}

function installApiFixtures(options?: {
  qualityMessage?: string
  qualitySuccess?: boolean
}) {
  const calls: string[] = []
  apiClient.get = async (url) => {
    calls.push(`GET ${url}`)
    if (url === '/api/proxy/') {
      return {
        data: {
          success: true,
          data: { items: [proxySummary], total: 1, page: 1, page_size: 100 },
        },
      }
    }
    throw new Error(`Unexpected GET ${url}`)
  }
  apiClient.post = async (url) => {
    calls.push(`POST ${url}`)
    if (url === '/api/proxy/1/quality-check') {
      return {
        data:
          options?.qualitySuccess === false
            ? { success: false, message: options.qualityMessage }
            : { success: true, data: summaryWithItems },
      }
    }
    if (url === '/api/proxy/1/test') {
      return { data: { success: true, data: proxySummary } }
    }
    throw new Error(`Unexpected POST ${url}`)
  }
  return { calls: () => calls }
}

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <Proxies />
    </QueryClientProvider>
  )
}

describe('Proxies page quality interactions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  test('opens quality report dialog with per-target items after clicking Quality', async () => {
    const { calls } = installApiFixtures()
    renderPage()
    const user = userEvent.setup()

    expect(await screen.findByText('p1')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Quality' }))

    expect(await screen.findByText('Quality Report')).toBeInTheDocument()
    expect(screen.getByText('OpenAI')).toBeInTheDocument()
    expect(
      screen.getByText('https://api.openai.com/v1/models')
    ).toBeInTheDocument()
    expect(calls()).toContain('POST /api/proxy/1/quality-check')
    expect(toast.error).not.toHaveBeenCalled()
  })

  test('runs test without opening dialog and without error toast on success', async () => {
    const { calls } = installApiFixtures()
    renderPage()
    const user = userEvent.setup()

    expect(await screen.findByText('p1')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Test' }))

    await waitFor(() => {
      expect(calls()).toContain('POST /api/proxy/1/test')
    })
    expect(screen.queryByText('Quality Report')).not.toBeInTheDocument()
    expect(toast.error).not.toHaveBeenCalled()
  })

  test('shows error toast and no dialog when quality check fails', async () => {
    installApiFixtures({ qualitySuccess: false, qualityMessage: '探测失败' })
    renderPage()
    const user = userEvent.setup()

    expect(await screen.findByText('p1')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Quality' }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('探测失败')
    })
    expect(screen.queryByText('Quality Report')).not.toBeInTheDocument()
  })
})