import {api} from '@/lib/api'
import {requireServerSuccess} from '@/lib/server-error-message'

export interface Counts {
    requests: number
    success: number
    failure: number
    rejected: number
    cancelled: number
    throttled: number
    retries: number
    tokens: number
    unknown_tokens: number
}

export interface Percentiles {
    count: number
    p50: number | null
    p90: number | null
    p95: number | null
    p99: number | null
    avg: number | null
    max: number | null
}

export interface Summary extends Counts {
    latency: Percentiles;
    ttft: Percentiles
}

export interface Snapshot {
    started_at: number
    generated_at: number
    start: number
    end: number
    step: number
    summary: Summary
    submissions: Summary
    tasks: Summary
    attempts: Counts
    trend: (Counts & { bucket: number })[]
    channels: (Counts & { id: number; name: string })[]
}

export interface OpsData {
    snapshot: Snapshot
    recent: Snapshot
    live: {
        in_flight: number
        channels: { id: number; name: string; in_flight: number }[]
        pending: number
        dropped: number
        last_write: number
        write_failed: boolean
    }
    system: {
        timestamp: number
        cpu: number | null
        memory: number | null
        memory_used: number
        memory_total: number
        go_memory: number
        goroutines: number
        db_ok: boolean | null
        db_open: number
        db_in_use: number
        db_idle: number
        db_max: number
        redis_enabled: boolean
        redis_ok: boolean | null
        redis_total: number
        redis_idle: number
    } | null
}

export interface OpsEvent {
    id: string
    request_id: string
    timestamp: number
    kind: string
    username: string
    user_id: number
    channel_id: number
    channel_name: string
    model_name: string
    group_name: string
    outcome: string
    status: number
    duration_ms: number
    ttft_ms: number | null
    attempt: number
    tokens: number
    tokens_known: boolean
    error_message: string
    error_truncated: boolean
}

export interface Filters {
    channel_id: string;
    model: string;
    group: string
}

export interface Range {
    start: number;
    end: number
}

export function monitoringRange(period: string, start: string, end: string, now = Date.now()): Range | null {
    const minute = Math.floor(now / 60000) * 60
    if (period !== 'custom') {
        const hours = Number(period)
        if (!Number.isFinite(hours) || hours <= 0 || hours > 720) return null
        return {start: minute + 60 - hours * 3600, end: minute + 60}
    }
    const range = {start: Date.parse(`${start}+08:00`) / 1000, end: Date.parse(`${end}+08:00`) / 1000}
    if (!Number.isFinite(range.start) || !Number.isFinite(range.end) || range.start < minute - 30 * 86400 || range.end > minute + 60 || range.end <= range.start || range.end - range.start > 30 * 86400 || range.start % 60 || range.end % 60) return null
    return range
}

export async function getOpsData(range: Range, filters: Filters, signal?: AbortSignal) {
    const response = await api.get<{
        success: boolean;
        data: OpsData
    }>('/api/ops-monitor', {params: {...range, ...filters}, signal})
    return requireServerSuccess(response.data).data
}

export async function getOpsEvents(params: Range & Filters & {
    kind: string;
    outcome: string;
    request_id: string;
    user_id: string;
    before?: number;
    before_id?: string
}, signal?: AbortSignal) {
    const response = await api.get<{
        success: boolean;
        data: { items: OpsEvent[]; has_more: boolean }
    }>('/api/ops-monitor/events', {params, signal})
    return requireServerSuccess(response.data).data
}

export function successRate(counts: Pick<Counts, 'success' | 'failure'>): number | null {
    const denominator = counts.success + counts.failure
    return denominator ? counts.success / denominator * 100 : null
}
