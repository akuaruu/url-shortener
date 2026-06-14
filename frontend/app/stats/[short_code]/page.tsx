'use client';

import { useEffect, useState, useCallback } from 'react';
import { useParams } from 'next/navigation';
import Link from 'next/link';
import {
    AreaChart,
    Area,
    XAxis,
    YAxis,
    Tooltip,
    ResponsiveContainer,
} from 'recharts';

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? 'https://url-s.aruu.app';
const POLL_INTERVAL_MS = 30_000;

// ─── Types
interface URLStats {
    original_url: string;
    created_at: string;
    expires_at: string | null;
    click_count: number;
}

interface ChartPoint {
    label: string;
    clicks: number;
}

// ─── Helpers 
function formatDate(iso: string): string {
    return new Date(iso).toLocaleString('en-GB', {
        day: '2-digit',
        month: 'short',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
    });
}

function truncate(url: string, max = 70): string {
    try {
        const { hostname, pathname } = new URL(url);
        const full = hostname + pathname;
        return full.length > max ? full.slice(0, max) + '…' : full;
    } catch {
        return url.length > max ? url.slice(0, max) + '…' : url;
    }
}

/**
 * Synthesizes a per-hour sparkline from a single click_count total.
 * Replace with a real /click-logs endpoint if you add one later.
 */
function buildSparkline(total: number, createdAt: string): ChartPoint[] {
    const now = Date.now();
    const start = new Date(createdAt).getTime();
    const hoursAlive = Math.max(1, Math.floor((now - start) / 3_600_000));
    const hours = Math.min(hoursAlive, 24);
    const points: ChartPoint[] = [];

    let distributed = 0;
    for (let i = 0; i < hours; i++) {
        const weight = i + 1;
        const share = Math.round((weight / ((hours * (hours + 1)) / 2)) * total);
        const safe = i === hours - 1 ? total - distributed : share;
        distributed += safe;
        const ts = new Date(start + i * 3_600_000);
        points.push({
            label: ts.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' }),
            clicks: safe,
        });
    }

    return points;
}

// ─── Sub-components 
function ChartTooltip({ active, payload, label }: any) {
    if (!active || !payload?.length) return null;
    return (
        <div className="bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-xs text-gray-200 shadow-lg">
            <p className="text-gray-400 mb-1">{label}</p>
            <p className="font-semibold text-emerald-400">{payload[0].value} clicks</p>
        </div>
    );
}

function StatCard({ label, value }: { label: string; value: string | number }) {
    return (
        <div className="bg-gray-900 border border-gray-800 rounded-xl px-5 py-4">
            <p className="text-xs text-gray-500 uppercase tracking-widest mb-1">{label}</p>
            <p className="text-xl font-bold text-white truncate">{value}</p>
        </div>
    );
}

// ─── Page 
export default function StatsPage() {
    const params = useParams();
    const shortCode = params?.short_code as string;

    const [stats, setStats] = useState<URLStats | null>(null);
    const [chart, setChart] = useState<ChartPoint[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState('');
    const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

    const fetchStats = useCallback(async (isInitial = false) => {
        if (isInitial) setLoading(true);
        setError('');

        try {
            const res = await fetch(`${API_BASE}/api/v1/urls/${shortCode}`);

            if (res.status === 404) throw new Error('Short URL not found.');
            if (res.status === 410) throw new Error('This short URL has expired.');
            if (!res.ok) throw new Error('Failed to fetch stats. Try again later.');

            const data: URLStats = await res.json();
            setStats(data);
            setChart(buildSparkline(data.click_count, data.created_at));
            setLastUpdated(new Date());
        } catch (err: any) {
            setError(err.message ?? 'Unexpected error.');
        } finally {
            if (isInitial) setLoading(false);
        }
    }, [shortCode]);

    // Initial fetch + polling every 30s
    useEffect(() => {
        if (!shortCode) return;

        fetchStats(true);

        const interval = setInterval(() => fetchStats(false), POLL_INTERVAL_MS);
        return () => clearInterval(interval);
    }, [shortCode, fetchStats]);

    // ── Loading
    if (loading) {
        return (
            <main className="min-h-screen bg-gray-950 flex items-center justify-center">
                <div className="flex flex-col items-center gap-3">
                    <div className="w-8 h-8 border-2 border-emerald-500 border-t-transparent rounded-full animate-spin" />
                    <p className="text-gray-500 text-sm">Fetching stats…</p>
                </div>
            </main>
        );
    }

    // ── Error
    if (error) {
        return (
            <main className="min-h-screen bg-gray-950 flex items-center justify-center p-4">
                <div className="max-w-md w-full bg-gray-900 border border-red-900/50 rounded-2xl p-8 text-center space-y-4">
                    <p className="text-red-400 font-semibold text-lg">Something went wrong</p>
                    <p className="text-gray-400 text-sm">{error}</p>
                    <Link
                        href="/"
                        className="inline-block px-5 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors"
                    >
                        ← Back to home
                    </Link>
                </div>
            </main>
        );
    }

    if (!stats) return null;

    const shortURL = `${API_BASE}/${shortCode}`;
    const isExpired = stats.expires_at ? new Date(stats.expires_at) < new Date() : false;

    return (
        <main className="min-h-screen bg-gray-950 text-gray-100 p-4 md:p-8">
            <div className="max-w-2xl mx-auto space-y-6">

                {/* Header nav */}
                <div className="flex items-center justify-between">
                    <Link
                        href="/"
                        className="text-sm text-gray-500 hover:text-gray-300 transition-colors"
                    >
                        ← Shorten another
                    </Link>
                    <span className="text-xs text-gray-600 font-mono">{shortCode}</span>
                </div>

                {/* Title block */}
                <div className="bg-gray-900 border border-gray-800 rounded-2xl p-6 space-y-4">
                    <div className="flex items-start justify-between gap-4">
                        <div className="min-w-0">
                            <p className="text-xs text-gray-500 uppercase tracking-widest mb-1">Short URL</p>
                            <a
                                href={shortURL}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="text-emerald-400 hover:text-emerald-300 font-mono text-sm transition-colors"
                            >
                                {shortURL}
                            </a>
                        </div>
                        {isExpired && (
                            <span className="shrink-0 text-xs px-2 py-1 bg-red-900/40 border border-red-800/50 text-red-400 rounded-md">
                                Expired
                            </span>
                        )}
                    </div>

                    <div>
                        <p className="text-xs text-gray-500 uppercase tracking-widest mb-1">Destination</p>
                        <a
                            href={stats.original_url}
                            target="_blank"
                            rel="noopener noreferrer"
                            title={stats.original_url}
                            className="text-gray-300 hover:text-white text-sm transition-colors break-all"
                        >
                            {truncate(stats.original_url)}
                        </a>
                    </div>
                </div>

                {/* Stat cards */}
                <div className="grid grid-cols-3 gap-3">
                    <StatCard label="Total Clicks" value={stats.click_count.toLocaleString()} />
                    <StatCard label="Created" value={formatDate(stats.created_at)} />
                    <StatCard
                        label="Expires"
                        value={stats.expires_at ? formatDate(stats.expires_at) : 'Never'}
                    />
                </div>

                {/* Chart */}
                <div className="bg-gray-900 border border-gray-800 rounded-2xl p-6">
                    <p className="text-xs text-gray-500 uppercase tracking-widest mb-5">
                        Estimated click distribution — last {chart.length}h
                    </p>

                    {stats.click_count === 0 ? (
                        <div className="h-36 flex items-center justify-center">
                            <p className="text-gray-600 text-sm">No clicks recorded yet.</p>
                        </div>
                    ) : (
                        <ResponsiveContainer width="100%" height={160}>
                            <AreaChart data={chart} margin={{ top: 4, right: 4, left: -28, bottom: 0 }}>
                                <defs>
                                    <linearGradient id="emeraldGrad" x1="0" y1="0" x2="0" y2="1">
                                        <stop offset="5%" stopColor="#10b981" stopOpacity={0.25} />
                                        <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                                    </linearGradient>
                                </defs>
                                <XAxis
                                    dataKey="label"
                                    tick={{ fill: '#4b5563', fontSize: 10 }}
                                    tickLine={false}
                                    axisLine={false}
                                    interval="preserveStartEnd"
                                />
                                <YAxis
                                    tick={{ fill: '#4b5563', fontSize: 10 }}
                                    tickLine={false}
                                    axisLine={false}
                                    allowDecimals={false}
                                />
                                <Tooltip content={<ChartTooltip />} />
                                <Area
                                    type="monotone"
                                    dataKey="clicks"
                                    stroke="#10b981"
                                    strokeWidth={2}
                                    fill="url(#emeraldGrad)"
                                    dot={false}
                                    activeDot={{ r: 4, fill: '#10b981', strokeWidth: 0 }}
                                />
                            </AreaChart>
                        </ResponsiveContainer>
                    )}
                </div>

                {/* Footer */}
                <p className="text-center text-xs text-gray-700">
                    {lastUpdated
                        ? `Last updated ${lastUpdated.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit' })} · refreshes every 30s`
                        : 'Refreshes every 30s'}
                </p>

            </div>
        </main>
    );
}