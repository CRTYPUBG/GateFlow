# GateFlow Cloudflare Edge Deployment Guide

## 1. Mimari Genel Bakış
GateFlow dağıtımı, Cloudflare Edge ağını ters proxy ve güvenlik katmanı olarak konumlandırır:

- **Frontend (Yönetim Paneli):** Cloudflare Pages üzerinde statik React SPA (`npm run build` -> `dist/`).
  - Domain: `app.example.com`
  - Build command: `npm run build`
  - Output directory: `dist`
- **Backend (Gateway Data & Control Plane):** Docker/Origin sunucusu üzerinde Go servisi.
  - Domain: `api.example.com`

---

## 2. Cloudflare Güvenlik ve Header Konfigürasyonu

### A. CF-Connecting-IP Doğrulaması
GateFlow, gelen istekteki `CF-Connecting-IP` başlığını **yalnızca** istek Cloudflare'ın resmi IP bloklarından (`173.245.48.0/20`, `104.16.0.0/13`, vb.) geliyorsa güvenilir istemci IP'si olarak kabul eder. Aksi halde başlık yok sayılır ve doğrudan TCP `RemoteAddr` kullanılır (Header Spoofing koruması).

### B. Authenticated Origin Pulls (mTLS)
Origin sunucusunun doğrudan internetten gelen istekleri reddetmesi ve yalnızca Cloudflare üzerinden gelen trafiği kabul etmesi için Cloudflare Authenticated Origin Pulls etkinleştirilmelidir:
1. Cloudflare Dashboard -> SSL/TLS -> Origin Server -> Authenticated Origin Pulls -> **On**.
2. Cloudflare CA sertifikası Origin sunucusuna yüklenir.

### C. Cloudflare Tunnel (Alternatif ve Önerilen)
Sunucu portlarını kamuya açık IP'ye açmadan origin bağlantısı sağlamak için `cloudflared` tüneli kullanılır:
```bash
cloudflared tunnel create gateflow-tunnel
cloudflared tunnel route dns gateflow-tunnel api.example.com
cloudflared tunnel run gateflow-tunnel
```
Bu sayede Origin üzerinde hiçbir inbound port açmaya gerek kalmaz.
