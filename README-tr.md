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
curl -fsSL https://raw.githubusercontent.com/deckplane/cli/main/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/deckplane/cli/main/install.ps1 | iex
```

**Go 1.26+ ile**

```bash
go install github.com/deckplane/cli@latest
```

Çalıştığını `deckplane version` ile kontrol edin.

Kurulum scriptleri indirilen dosyanın checksum'ını doğrular. Belirli bir sürümü kurmak için çalıştırmadan önce `DECKPLANE_VERSION=v0.1.0`, klasörü seçmek için `DECKPLANE_INSTALL_DIR` ayarlayın. CLI'ı güncellemek için kurulum scriptini istediğiniz zaman tekrar çalıştırmanız yeterli.

## Deckplane Core'u kurun

```bash
sudo deckplane core install
```

Kurulum sihirbazı sırasıyla:

1. Docker'ı kontrol eder,
2. lisans anahtarınızı sorar (yazarken gizlenir) ve Deckplane ile doğrular,
3. Core'a nasıl erişileceğini sorar: HTTPS ile kendi alan adınız ya da bu sunucunun IP adresi (aşağıya bakın),
4. tüm şifreleri ve gizli anahtarları sizin için üretir,
5. Core'u ve veritabanını indirip başlatır, sağlıklı hale gelmesini bekler,
6. ilk yönetici hesabınızı oluşturmayı önerir.

Sonunda açmanız gereken adresi gösterir. `install` komutunu tekrar çalıştırmak güvenlidir; verileriniz ve gizli anahtarlarınız korunur.

### Alan adı ya da IP adresi

Core hiçbir zaman kendi başına port açmaz; her zaman [Traefik](https://traefik.io) arkasında çalışır. Sunucuda Docker içinde çalışan bir Traefik varsa kurulum onu bulur, ağını, sertifika çözücüsünü ve entrypoint'lerini okur, sonra onayınızı ister. Yoksa Deckplane kendi Traefik'ini kurar.

| Seçim | Sonuç | Gerekenler |
|---|---|---|
| **Kendi alan adınız** (önerilen) | Sertifikalı `https://deckplane.example.com` | Alan adının DNS kaydının bu sunucuyu göstermesi. Kendi Traefik'inizle başka bir şey gerekmez; Deckplane'in Traefik'iyle 80 ve 443 portlarının boş olması ve Let's Encrypt için bir e-posta adresi |
| **Alan adı olmadan, IP adresi** | `http://203.0.113.10` (80 portunda düz HTTP) | 80 portunun boş olması ya da kendi Traefik'inizin HTTP entrypoint'i. Deckplane'i denemek ya da iç ağlar için uygundur |

Kendi Traefik'iniz varken alan adı vermezseniz Core, 80 portuna gelen ve başka hiçbir uygulamanın almadığı tüm istekleri karşılar. Traefik'iniz tüm HTTP isteklerini HTTPS'e yönlendiriyorsa bu çalışmaz; o durumda alan adı kullanın.

Deckplane kendi Traefik'ini kurduğunda sunucudaki diğer uygulamalar da onu kullanabilir: onları `deckplane-proxy` Docker ağına bağlayın ve Traefik label'ları ekleyin (alan adıyla `websecure` entrypoint'i ve `le` sertifika çözücüsü, alan adı olmadan `web` entrypoint'i).

> Linux'ta Core `/var/lib/deckplane` klasörüne kurulduğu için `sudo` gerekir. macOS ve Windows'ta `~/.deckplane/core` klasörüne kurulur ve `sudo` gerekmez.

### Soru sormadan (otomasyon)

```bash
export DECKPLANE_LICENSE_KEY=dpl_...
export DECKPLANE_ADMIN_PASSWORD='guclu-bir-sifre'
sudo -E deckplane core install --yes \
  --domain deckplane.example.com --acme-email siz@example.com \
  --admin-email admin@example.com --admin-name "Admin"
```

Çalışan bir Traefik varsa bulunur ve kullanılır; o zaman `--acme-email` gerekmez. Yoksa Deckplane kendi Traefik'ini kurar. Tespit Traefik'inizin ayarlarını okuyamazsa bunları `--traefik-network`, `--cert-resolver` ve `--traefik-entrypoint` ile verin. Core'u sunucunun IP adresinden HTTP ile yayınlamak için `--domain` ve `--acme-email` seçeneklerini çıkarın.

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
- **"Port 80 (veya 443) is already in use"**: Deckplane kendi Traefik'ini kurmak istiyor ama portu başka bir program kullanıyor; mesaj, mümkünse hangi program olduğunu söyler. Docker içinde çalışan bir Traefik ise onu `--traefik-network <ağının adı>` ile kullanın. Değilse o programı durdurun (örneğin `systemctl stop nginx`) ve kurulumu tekrar çalıştırın.
- **IP adresi açılmıyor** (alan adı yok, kendi Traefik'iniz var): Traefik'iniz muhtemelen tüm HTTP isteklerini HTTPS'e yönlendiriyor. Bunun yerine alan adı kullanın.
- **Alan adı açılmıyor ya da sertifika uyarısı veriyor**: Alan adının DNS A kaydı bu sunucuyu göstermeli ve Let's Encrypt'in sertifika verebilmesi için 80 ve 443 portları internetten erişilebilir olmalı.
- **Core sağlıklı hale gelmiyor**: Nedenini görmek için `deckplane core logs` çalıştırın.
- Diğer her şey için `deckplane doctor` çalıştırın, ardından çıktısıyla birlikte bir [issue açın](https://github.com/deckplane/cli/issues).

## Geliştirme

```bash
go build -o deckplane .
go test ./...
./deckplane --help
```

Yerelde derlenmiş bir Core imajıyla test etmek için `DECKPLANE_DEV=1` ayarlayın. Bu ayar registry girişini atlar ve imaj çekme hatalarını yok sayar. Böylece örneğin `localhost/deckplane/core:dev` etiketli bir imajla `--registry localhost --version dev` kullanabilirsiniz.

### Sürüm yayınlama

Bir sürüm etiketi gönderdiğinizde GitHub Actions; Linux, macOS ve Windows (amd64 ve arm64) için binary'leri bir `checksums.txt` ile birlikte yayınlar:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Bir sürümü yerelde denemek için `goreleaser release --snapshot --clean` kullanın.

## Kommando ile geliştirildi

Komut yapısı, Go için minimalist bir CLI kütüphanesi olan [Kommando](https://github.com/yigit433/kommando) ile kurulmuştur.

## Katkıda bulunma

Hata bildirimlerine ve pull request'lere açığız. Ayrıntılar [CONTRIBUTING.md](CONTRIBUTING.md) dosyasında. Güvenlik açıklarını [SECURITY.md](SECURITY.md) dosyasında anlatıldığı gibi gizli olarak bildirin.

## Lisans

[Apache Lisansı 2.0](LICENSE) © 2026 Meya Teknoloji
