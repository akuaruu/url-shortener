import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';

// --- CUSTOM METRICS ---
// Kita pisahkan metrik ini agar mudah dibaca di terminal nanti
export const redirectLatency = new Trend('redirect_duration');
export const successRate = new Rate('successful_redirects');

// --- KONFIGURASI SIKSAAN (STAGES) ---
export const options = {
    // Skenario bertahap agar lebih realistis seperti lonjakan trafik asli
    stages: [
        { duration: '10s', target: 1 },  // Pemanasan: Naik ke 20 VU (sesuai limit Supabase)
        { duration: '30s', target: 5 }, // Puncak: Hajar dengan 100 VU secara bersamaan
        { duration: '10s', target: 0 },   // Pendinginan: Turun perlahan ke 0 VU
    ],
    thresholds: {
        // Ini adalah KPI (Key Performance Indicator) targetmu!
        // Jika latensi 95% request tembus di atas 50ms, k6 akan memberikan status FAIL.
        'redirect_duration': ['p(95)<80'],
        'successful_redirects': ['rate>0.99'], // 99% request harus tidak error (harus status 302)
    },
};

// --- FASE SETUP (Dijalankan 1x sebelum VU menyerang) ---
export function setup() {

    //const url = 'https://url-s.aruu.app/api/v1/shorten';
    //Local testing :
    const url = 'http://localhost:8080/api/v1/shorten';

    const payload = JSON.stringify({
        original_url: 'https://aruu.app/portfolio',
        ttl_seconds: 86400
    });

    const params = {
        headers: { 'Content-Type': 'application/json' },
    };

    const res = http.post(url, payload, params);

    if (res.status !== 201 && res.status !== 200) {
        console.error(`Setup GAGAL! Pastikan gateway nyala. Status: ${res.status}, Body: ${res.body}`);
    }

    // Mengambil short_code dari respons JSON
    const shortCode = JSON.parse(res.body).short_code;
    console.log(`[Setup OK] Berhasil membuat URL pendek: ${shortCode}. Memulai siksaan...`);

    return { shortCode: shortCode }; // Oper data ini ke pasukan VU
}

// --- FASE UTAMA (Dijalankan ribuan kali oleh pasukan VU) ---
export default function (data) {
    // Pasukan VU akan menembak short_code yang dibuat di fase setup
    //const url = `http://url-s.aruu.app/${data.shortCode}`;

    // Local testing:
    const url = `http://localhost:8080/${data.shortCode}`;

    // PENTING: redirects: 0 agar k6 tidak ikut pindah ke URL asli
    const res = http.get(url, { redirects: 0 });

    // Mencatat metrik kustom
    redirectLatency.add(res.timings.duration);
    successRate.add(res.status === 302);

    // Validasi respons harus 302 dan memiliki header Location
    check(res, {
        'is status 302': (r) => r.status === 302,
        'has location header': (r) => r.headers['Location'] !== undefined,
    });

    // mimicking human
    sleep(0.05);
}