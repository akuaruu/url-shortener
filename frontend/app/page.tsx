'use client';

import { useState } from 'react';
import Link from 'next/link';

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? 'https://url-s.aruu.app';

interface ShortenResult {
  shortCode: string;
  shortUrl: string;
}

export default function Home() {
  const [url, setUrl] = useState('');
  const [result, setResult] = useState<ShortenResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [copied, setCopied] = useState(false);

  const handleShorten = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!url) return;

    setLoading(true);
    setError('');
    setResult(null);
    setCopied(false);

    try {
      const response = await fetch(`${API_BASE}/api/v1/shorten`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          original_url: url,
          ttl_seconds: 864000,
        }),
      });

      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body?.message ?? 'Failed to shorten URL. Check your API connection.');
      }

      const data = await response.json();
      setResult({
        shortCode: data.short_code,
        shortUrl: `${API_BASE}/${data.short_code}`,
      });
    } catch (err: any) {
      setError(err.message || 'An unexpected error occurred.');
    } finally {
      setLoading(false);
    }
  };

  const copyToClipboard = () => {
    if (!result) return;
    navigator.clipboard.writeText(result.shortUrl);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleShortenAnother = () => {
    setResult(null);
    setUrl('');
    setError('');
  };

  return (
    <main className="min-h-screen bg-gray-950 flex items-center justify-center p-4 font-sans text-gray-100">
      <div className="max-w-xl w-full space-y-8 bg-gray-900 p-8 rounded-2xl shadow-2xl border border-gray-800">

        {/* Header */}
        <div className="text-center space-y-2">
          <h1 className="text-4xl font-extrabold tracking-tight text-white">
            <span className="text-emerald-500">Fast</span>URL
          </h1>
          <p className="text-gray-400 text-sm">
            High-performance URL shortener powered by Go & Redis.
          </p>
        </div>

        {/* Form — hidden after success */}
        {!result && (
          <form onSubmit={handleShorten} className="space-y-4">
            <input
              type="url"
              required
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="https://your-long-url.com..."
              className="w-full px-5 py-4 bg-gray-950 border border-gray-700 rounded-xl focus:ring-2 focus:ring-emerald-500 focus:border-transparent outline-none transition-all placeholder-gray-500"
            />
            <button
              type="submit"
              disabled={loading}
              className={`w-full py-4 rounded-xl font-bold text-lg transition-all ${loading
                  ? 'bg-gray-700 cursor-not-allowed text-gray-400'
                  : 'bg-emerald-600 hover:bg-emerald-500 text-white shadow-lg hover:shadow-emerald-500/20'
                }`}
            >
              {loading ? 'Shortening…' : 'Shorten URL'}
            </button>
          </form>
        )}

        {/* Error state */}
        {error && (
          <div className="p-4 bg-red-900/30 border border-red-500/50 rounded-xl text-red-400 text-center text-sm">
            {error}
          </div>
        )}

        {/* Success state */}
        {result && (
          <div className="space-y-4">
            <div className="p-1 bg-gradient-to-r from-emerald-500 to-teal-500 rounded-xl">
              <div className="bg-gray-950 p-6 rounded-lg flex flex-col items-center space-y-4">
                <p className="text-sm text-gray-400 font-medium">Your short URL is ready!</p>
                <a
                  href={result.shortUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-2xl font-bold text-emerald-400 hover:text-emerald-300 transition-colors break-all text-center"
                >
                  {result.shortUrl}
                </a>

                {/* Action buttons */}
                <div className="flex gap-3 w-full">
                  <button
                    onClick={copyToClipboard}
                    className="flex-1 px-4 py-2.5 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg font-medium transition-colors border border-gray-700 text-sm"
                  >
                    {copied ? '✅ Copied!' : 'Copy URL'}
                  </button>
                  <Link
                    href={`/stats/${result.shortCode}`}
                    className="flex-1 px-4 py-2.5 bg-emerald-900/40 hover:bg-emerald-900/60 text-emerald-400 hover:text-emerald-300 rounded-lg font-medium transition-colors border border-emerald-800/50 text-sm text-center"
                  >
                    View Stats →
                  </Link>
                </div>
              </div>
            </div>

            {/* Shorten another */}
            <button
              onClick={handleShortenAnother}
              className="w-full py-3 text-sm text-gray-500 hover:text-gray-300 transition-colors"
            >
              ← Shorten another URL
            </button>
          </div>
        )}

      </div>
    </main>
  );
}