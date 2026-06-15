import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';

// ─── CUSTOM METRICS 
export const redirectLatency = new Trend('redirect_duration');
export const successRate = new Rate('successful_redirects');

// ─── ENVIRONMENT 
// Usage:
//   Local baseline : k6 run -e TEST=baseline loadtest.js
//   Local stress   : k6 run -e TEST=stress   loadtest.js
//   Production     : k6 run -e TEST=prod     loadtest.js
const TEST = __ENV.TEST || 'baseline';
const IS_PROD = TEST === 'prod';
const BASE_URL = IS_PROD
    ? 'https://url-s.aruu.app'
    : 'http://localhost:8080';

// ─── STAGES & THRESHOLDS per TEST
const CONFIGS = {
    baseline: {
        stages: [
            { duration: '15s', target: 10 },   // ramp up
            { duration: '30s', target: 50 },   // hold
            { duration: '10s', target: 0 },   // cool down
        ],
        thresholds: {
            'redirect_duration': ['p(95)<10'],    // Redis cache hit harusnya <10ms lokal
            'successful_redirects': ['rate>0.99'],
        },
    },
    stress: {
        stages: [
            { duration: '10s', target: 50 },
            { duration: '30s', target: 200 },
            { duration: '30s', target: 500 },  // titik sistem mulai kesakitan
            { duration: '15s', target: 0 },
        ],
        thresholds: {
            'redirect_duration': ['p(95)<50'],    // sedikit longgar untuk stress
            'successful_redirects': ['rate>0.95'],
        },
    },
    prod: {
        stages: [
            { duration: '15s', target: 10 },
            { duration: '30s', target: 50 },
            { duration: '30s', target: 100 },  // spike moderat, ada network latency
            { duration: '15s', target: 0 },
        ],
        thresholds: {
            'redirect_duration': ['p(95)<100'],   // realistis dengan network + Cloudflare
            'successful_redirects': ['rate>0.99'],
        },
    },
};

export const options = CONFIGS[TEST];

// ─── SETUP (dijalankan 1x sebelum VU menyerang
export function setup() {
    const url = `${BASE_URL}/api/v1/shorten`;

    // Didefinisikan di sini agar tidak ReferenceError
    const payload = JSON.stringify({
        original_url: 'https://aruu.app/portfolio',
        ttl_seconds: 86400,
    });
    const params = {
        headers: { 'Content-Type': 'application/json' },
    };

    // Buat 10 short code berbeda agar hit pattern lebih realistis
    const codes = [];
    for (let i = 0; i < 10; i++) {
        const res = http.post(url, payload, params);
        if (res.status !== 201 && res.status !== 200) {
            console.error(`[Setup] GAGAL iterasi ${i} — status: ${res.status}, body: ${res.body}`);
            continue;
        }
        const body = JSON.parse(res.body);
        codes.push(body.short_code);
    }

    if (codes.length === 0) {
        throw new Error('[Setup] Tidak ada short code yang berhasil dibuat. Pastikan gateway nyala.');
    }

    console.log(`[Setup OK] ${codes.length} short codes: [${codes.join(', ')}] | ENV=${IS_PROD ? 'production' : 'local'} | TEST=${TEST}`);
    return { codes };
}

// ─── MAIN (dijalankan ribuan kali oleh pasukan VU) 
export default function (data) {
    // Randomize agar setiap VU tidak selalu hit key yang sama
    const code = data.codes[Math.floor(Math.random() * data.codes.length)];
    const url = `${BASE_URL}/${code}`;

    // redirects: 0 — k6 tidak ikut redirect, kita ukur response 302-nya
    const res = http.get(url, { redirects: 0 });

    redirectLatency.add(res.timings.duration);
    successRate.add(res.status === 302);

    check(res, {
        'is status 302': (r) => r.status === 302,
        'has location header': (r) => r.headers['Location'] !== undefined,
    });

    sleep(0.05); // mimicking human think time
}