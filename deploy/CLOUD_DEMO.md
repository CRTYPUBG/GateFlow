# GateFlow Bulut Canlı Demo Dağıtım Kılavuzu (Ücretsiz Seçenekler)

GateFlow'u kendi bilgisayarınızda çalıştırmak zorunda kalmadan, internete açık gerçek bir adreste canlı olarak ayağa kaldırmak için aşağıdaki ücretsiz ve doğrudan GitHub entegrasyonu sunan platformları kullanabilirsiniz:

---

## 🌟 1. Seçenek: Render.com (En Hızlı ve Önerilen - 1 Tıkla Dağıtım)

Deponuzda hazır `render.yaml` ve `Dockerfile` mevcuttur.

1. **[render.com](https://render.com)** adresine gidin ve GitHub hesabınızla giriş yapın.
2. Sağ üstten **New +** -> **Web Service** seçin.
3. GitHub deponuzu seçin: `CRTYPUBG/GateFlow`
4. Ayarlar:
   - **Environment:** Docker
   - **Plan:** Free
   - **Environment Variables:**
     - `GATEFLOW_ADMIN_KEY`: `admin-secret-gateflow` (veya istediğiniz güçlü bir anahtar)
     - `GATEFLOW_ALLOW_LOCAL_DEV`: `true`
5. **Create Web Service** butonuna tıklayın.
6. Yaklaşık 2 dakika içinde size bir genel adres verecektir (örneğin: `https://gateflow-xxxx.onrender.com`).

**Canlı Test:**
```bash
# Canlı sağlık kontrolü:
curl -i https://gateflow-xxxx.onrender.com/health/live

# Canlı proxy rotası:
curl -i https://gateflow-xxxx.onrender.com/anything/test
```

---

## 🚀 2. Seçenek: Koyeb (Ücretsiz & Docker Uyumlu)

1. **[koyeb.com](https://koyeb.com)** adresine gidin.
2. **Create App** -> **GitHub** seçeneğinden `CRTYPUBG/GateFlow` seçin.
3. Builder olarak **Dockerfile** seçin.
4. Port olarak `8080` belirtin.
5. **Deploy** butonuna tıklayın. Birkaç dakika içinde global HTTPS adresi atanır.

---

## ⚡ 3. Seçenek: Fly.io

Terminal veya GitHub Actions üzerinden `flyctl` kullanarak Frankfurt veya Amsterdam bölgesinde ücretsiz mikro container olarak çalıştırabilirsiniz:
```bash
fly launch --image gateflow:latest
```

---

## 💻 4. Seçenek: GitHub Codespaces (Tarayıcıda Anında Çalıştırma)

Hiçbir servis hesabı açmadan doğrudan GitHub üzerinde geçici bir sanal makinede çalıştırmak için:
1. GitHub deponuza gidin: **[https://github.com/CRTYPUBG/GateFlow](https://github.com/CRTYPUBG/GateFlow)**
2. Yeşil **`Code`** butonuna tıklayın -> **`Codespaces`** sekmesine geçin.
3. **`Create codespace on main`** butonuna basın.
4. Tarayıcınızda bulut VS Code açılacaktır. Terminale:
   ```bash
   go run ./cmd/gateway
   ```
   yazın.
5. Codespaces otomatik olarak `8080` portunu genel/açık bir HTTPS adresine iletir (`Ports` sekmesinden kopyalayabilirsiniz).

---

## 🌐 5. React Dashboard (Frontend) Dağıtımı: Cloudflare Pages

Yönetim panelini Cloudflare Pages üzerinde ücretsiz yayınlamak için:
1. **[Cloudflare Dashboard](https://dash.cloudflare.com)** -> **Workers & Pages** -> **Create Application** -> **Pages** -> **Connect to Git**.
2. `CRTYPUBG/GateFlow` reposunu seçin.
3. Yapılandırma:
   - **Framework preset:** `Vite`
   - **Root directory:** `frontend`
   - **Build command:** `npm run build`
   - **Output directory:** `dist`
   - **Environment Variables:**
     - `VITE_ADMIN_URL`: Bulutta çalışan backend adresiniz (örnek: `https://gateflow-xxxx.onrender.com`)
     - `VITE_ADMIN_KEY`: `admin-secret-gateflow`
4. **Save and Deploy** butonuna tıklayın. Panelinize `https://gateflow.pages.dev` adresinden erişebilirsiniz.
