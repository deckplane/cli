# Deckplane CLI

[English](README.md) · **Türkçe**

`deckplane` komutu, [Deckplane](https://deckplane.com)'i kendi sunucularınıza kurar ve yönetir. **Deckplane Core**'u (kendi sunucunuzda çalışan kontrol paneli) ve **Deckplane agent**'ı Docker ile kurar. İhtiyaç duyduğu her şeyi kurulum sırasında sorar, bu yüzden kod ya da Docker bilgisi gerekmez.

```bash
deckplane core install --license-key dpl_...
```

## Gerekenler

- Bir Linux sunucu (Ubuntu, Debian vb.) ya da Docker Desktop kurulu bir bilgisayar (macOS, Windows)
- Compose eklentisiyle birlikte [Docker](https://docs.docker.com/get-docker/). Linux'ta: `curl -fsSL https://get.docker.com | sudo sh`
- [Deckplane Cloud](https://cloud.deckplane.com) → **Lisanslar** sayfasından alınmış bir lisans anahtarı (`dpl_` ile başlar)

Makinenin hazır olup olmadığından emin değilseniz `deckplane doctor` çalıştırın. Her şeyi kontrol eder ve eksik olanı nasıl düzelteceğinizi söyler.

## CLI'ı kurun

**Linux ve macOS**

```bash
curl -fsSL https://raw.githubusercontent.com/deckplane/deckplane-cli/main/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/deckplane/deckplane-cli/main/install.ps1 | iex
```

**Go 1.26+ ile**

```bash
go install github.com/deckplane/deckplane-cli@latest
```

Çalıştığını `deckplane version` ile kontrol edin.

## Deckplane Core'u kurun

```bash
sudo deckplane core install
```

Kurulum sihirbazı sırasıyla:

1. Docker'ı kontrol eder,
2. lisans anahtarınızı sorar (yazarken gizlenir) ve Deckplane ile doğrular,
3. Core'a nasıl erişileceğini sorar: bu makinedeki bir port (en kolayı) ya da Traefik üzerinden kendi alan adınız,
4. tüm şifreleri ve gizli anahtarları sizin için üretir,
5. Core'u ve veritabanını indirip başlatır, sağlıklı hale gelmesini bekler,
6. ilk yönetici hesabınızı oluşturmayı önerir.

Sonunda açmanız gereken adresi gösterir. `install` komutunu tekrar çalıştırmak güvenlidir; verileriniz ve gizli anahtarlarınız korunur.

> Linux'ta Core `/var/lib/deckplane` klasörüne kurulduğu için `sudo` gerekir. macOS ve Windows'ta `~/.deckplane/core` klasörüne kurulur ve `sudo` gerekmez.

### Soru sormadan (otomasyon)

```bash
export DECKPLANE_LICENSE_KEY=dpl_...
export DECKPLANE_ADMIN_PASSWORD='guclu-bir-sifre'
sudo -E deckplane core install --yes --port 3000 \
  --admin-email admin@example.com --admin-name "Admin"
```

Core'u mevcut bir Traefik üzerinden kendi alan adınızda yayınlamak için `--port` yerine `--domain deckplane.example.com --traefik-network traefik` kullanın.

## Sunucu bağlayın (agent)

Yönetmek istediğiniz her Docker sunucusunda şunu çalıştırın:

```bash
sudo deckplane agent install --server-url https://deckplane.example.com --token <token>
```

Token'ı Core'un kurulu olduğu makinede `deckplane core token` ile görebilirsiniz. Core aynı makinede çalışıyorsa `deckplane agent install` onu kendisi bulur.

## Komutlar

| Komut | Ne yapar |
|---|---|
| `deckplane core install` | Deckplane Core'u kurar |
| `deckplane core status` | Core çalışıyor mu, adresi ve sürümü nedir gösterir |
| `deckplane core logs [-f]` | Core loglarını gösterir (`-f` canlı takip eder) |
| `deckplane core start` / `stop` / `restart` | Core'u başlatır, durdurur veya yeniden başlatır |
| `deckplane core update [--version v1.2.3]` | En son ya da belirtilen sürüme günceller |
| `deckplane core setup` | İlk yönetici hesabını oluşturur |
| `deckplane core token` | Agent'ların bağlanırken kullandığı token'ı yazdırır |
| `deckplane core config` | Ayarları gösterir (gizli değerler maskelenir; `--reveal` açar) |
| `deckplane core config set ANAHTAR=DEĞER --restart` | Bir ayarı değiştirir ve uygular |
| `deckplane core license dpl_...` | Lisans anahtarını değiştirir |
| `deckplane core uninstall [--remove-data]` | Core'u kaldırır; `--remove-data` verilmezse veriler korunur |
| `deckplane agent install` | Bu Docker sunucusunu Core'a bağlar |
| `deckplane agent status` / `logs` / `update` / `uninstall` | Agent'ı yönetir |
| `deckplane doctor` | Makinenin hazır olup olmadığını kontrol eder |
| `deckplane version` | CLI sürümünü yazdırır |
| `deckplane completion bash\|zsh\|fish\|powershell` | Kabuk tamamlama betiği üretir |

Herhangi bir komutun seçeneklerini görmek için sonuna `--help`, Docker çıktısının tamamını görmek için `-v` ekleyin. Başka bir klasördeki kurulumla çalışmak için tüm komutlar `--dir` seçeneğini kabul eder.

## Dosyalar nerede

| | Linux | macOS / Windows |
|---|---|---|
| Core | `/var/lib/deckplane` | `~/.deckplane/core` |
| Agent | `/var/lib/deckplane-agent` | `~/.deckplane/agent` |

Her klasörde bir `.env` dosyası (ayarlar ve gizli anahtarlar; yalnızca sahibi okuyabilir) ve otomatik üretilen bir `compose.yaml` bulunur. `compose.yaml` dosyasını düzenlemeyin, çünkü kurulum ve güncellemede yeniden yazılır. Kendi Compose değişikliklerinizi yanındaki `compose.override.yaml` dosyasına yazın; Docker Compose bunları otomatik birleştirir.

## Sorun giderme

- **"Docker is not installed" veya "not running"**: Mesaj, çalıştırmanız gereken komutu ya da açmanız gereken uygulamayı söyler.
- **"Your license key was not accepted"**: Deckplane Cloud → Lisanslar sayfasında anahtarın aktif olduğunu, süresinin dolmadığını ya da iptal edilmediğini ve anahtarın tamamını kopyaladığınızı kontrol edin.
- **"Port 3000 is already in use"**: Başka bir portla kurun, örneğin `--port 8080`.
- **Core sağlıklı hale gelmiyor**: Nedenini görmek için `deckplane core logs` çalıştırın.
- Diğer her şey için `deckplane doctor` çalıştırın, ardından çıktısıyla birlikte bir [issue açın](https://github.com/deckplane/deckplane-cli/issues).

## Geliştirme

```bash
go build -o deckplane .
go test ./...
./deckplane --help
```

Yerelde derlenmiş bir Core imajıyla test etmek için `DECKPLANE_DEV=1` ayarlayın. Bu ayar registry girişini atlar ve imaj çekme hatalarını yok sayar. Böylece örneğin `localhost/deckplane/core:dev` etiketli bir imajla `--registry localhost --version dev` kullanabilirsiniz.

## Kommando ile geliştirildi

Komut yapısı, Go için minimalist bir CLI kütüphanesi olan [Kommando](https://github.com/yigit433/kommando) ile kurulmuştur.

## Lisans

[Apache Lisansı 2.0](LICENSE) © 2026 Meya Teknoloji
