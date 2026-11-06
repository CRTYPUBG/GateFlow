# GateFlow — API Gateway and Traffic Management Platform

GateFlow, gerçek HTTP trafiğini yönlendiren, güvenlik politikalarını uygulayan, dağıtık rate limiting ve response caching sunan, yüksek performanslı ve modern bir API Gateway platformudur.

---

## 🚀 Özellikler

1. **Yüksek Performanslı Reverse Proxy (Data Plane):** Go `net/http` ve `httputil.ReverseProxy` (Rewrite tabanlı) motoru ile milisaniyeler altı yönlendirme.
2. **11 Aşamalı İstek Pipeline'ı:**
   - Protokol & Request ID atama (`X-Request-ID`)
   - Güvenilir proxy ve Cloudflare IP çözümlemesi (`CF-Connecting-IP` spoofing koruması)
   - CIDR ve IP Allow/Deny Access Control (ACL)
   - Deterministik ve öncelik bazlı Route eşleme (Desteklenmeyen isteklerde `405 Method Not Allowed`, bulunamayanlarda `404`)
   - Kriptografik API Key doğrulama (SHA-256 hash saklama, constant-time compare)
   - Token Bucket Rate Limiting (Redis atomik Lua sayaçları + lokal Memory fallback)
   - Akıllı Response Caching (Normalleştirilmiş query key'leri, ETag ve ETag 304, Auth & Set-Cookie sızıntı koruması)
   - Load Balancing (Round Robin, Weighted RR, Least Connections)
   - SSRF Koruması (Loopback, link-local ve AWS/GCP 169.254.169.254 metadata bloklama)
   - Aktif Sağlık Kontrolleri & Circuit Breaker
   - Prometheus Metrikleri (`/metrics`) ve Yapılandırılmış İstek Kayıtları
3. **Yönetim Paneli (Control Plane & UI):**
   - React + TypeScript + Vite modern koyu tema dashboard
   - Route, Upstream, API Key ve Politika yönetimi
   - Canlı trafik izleme ve Denetim (Audit) logları

---

## 🛠️ Yerel Çalıştırma Kılavuzu

### Seçenek 1: Docker Compose ile Başlatma (Önerilen)

```bash
# Servisleri başlat (Gateway, PostgreSQL, Redis, Prometheus, Grafana)
docker compose up -d

# Logları takip et
docker compose logs -f gateway
```

Servis erişim adresleri:
- **Gateway Traffic Engine:** `http://localhost:8080`
- **Control Plane API:** `http://localhost:8081`
- **Prometheus Metrikleri:** `http://localhost:8080/metrics` ve `http://localhost:9090`
- **Grafana:** `http://localhost:3000` (Giriş: admin / admin)

---

### Seçenek 2: Doğrudan Go & Node.js ile Çalıştırma (Zero-Dependency Modu)

GateFlow, harici bir veritabanı veya Redis olmadan da tam fonksiyonel çalışan dahili Memory Storage ve Memory Rate Limiter / Cache fallback mekanizmasına sahiptir.

#### Windows (PowerShell):
```powershell
# 1. Gateway servisini başlatın:
& "C:\Program Files\Go\bin\go.exe" run ./cmd/gateway

# 2. Yeni bir terminalde React Dashboard'u başlatın:
cd frontend
npm run dev
```

#### Linux / macOS:
```bash
# 1. Gateway servisini başlatın:
go run ./cmd/gateway

# 2. Yeni bir terminalde React Dashboard'u başlatın:
cd frontend
npm run dev
```

---

## 🧪 Testleri Çalıştırma

Tüm birim testleri, güvenlik testleri ve uçtan uca entegrasyon testlerini çalıştırmak için:

```powershell
# Windows
& "C:\Program Files\Go\bin\go.exe" test -v ./...

# Linux / macOS
go test -v ./...
```

---

## 🎬 Codejam Canlı Doğrulama & Demo Senaryosu

GateFlow çalışırken aşağıdaki adımlarla tüm özellikleri canlı HTTP trafiği üzerinde test edebilirsiniz:

### 1. Sağlık Endpoint Kontrolleri
```bash
curl -i http://localhost:8080/health/live
curl -i http://localhost:8080/health/ready
```

### 2. Canlı Route Üzerinden İstek (Public Route)
Varsayılan seed ile `/anything/*` rotası tanımlıdır:
```bash
curl -i http://localhost:8080/anything/test-flow
```
*İlk istek:* `X-Cache: MISS` yanıt başlığı döner ve istek upstream'e iletilir.  
*İkinci istek:* `X-Cache: HIT` başlığıyla doğrudan önbellekten milisaniyeler altında servis edilir.

### 3. Rate Limit Testi (`429 Too Many Requests`)
Aynı endpoint'e arka arkaya 6 kez istek gönderildiğinde 5 istek sonrasında sınır aşılır:
```bash
for i in {1..6}; do curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/anything/test-flow; done
```
*Sonuç:* `429` kodu ve `Retry-After` başlığı döner.

### 4. API Key Güvenlik Doğrulaması
`/headers` rotası API key doğrulaması gerektirir:
```bash
# Anahtarsız istek -> 401 Unauthorized
curl -i http://localhost:8080/headers

# Yönetim API'sinden yeni anahtar üretin:
curl -X POST http://localhost:8081/api/v1/keys \
  -H "X-Admin-Key: admin-secret-gateflow" \
  -H "Content-Type: application/json" \
  -d '{"owner":"DemoClient","description":"Test key","scopes":["read"]}'

# Dönen "raw_key" ile korunan rotayı çağırın:
curl -i -H "X-API-Key: <URETILEN_RAW_KEY>" http://localhost:8080/headers
```

### 5. Prometheus Metriklerini İnceleme
```bash
curl -s http://localhost:8080/metrics | grep gateflow_
```
Tüm istek sayıları, 2xx/4xx/5xx dağılımı, rate limit blokları ve cache istatistikleri gerçek zamanlı izlenir.
