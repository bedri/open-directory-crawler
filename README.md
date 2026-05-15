# ODK — Open Directory Crawler

Open directory'leri keşfeden, tarayan, kategorize eden ve isteğe bağlı Google Drive'a yedekleyen bir araç.

## Özellikler

- **Crawl** — HTTP ve FTP open directory'leri tara, dosyaları bul, kategorize et
- **Discover** — Google, Bing, DuckDuckGo, Shodan, SearXNG ile open directory keşfi
- **Kategorizasyon** — video, audio, image, document, archive, code, executable, torrent, other
- **Dual DB** — BadgerDB: yazıcı (agent) + okuyucu (API/CLI), her cycle sonunda sync
- **API** — REST API ile sorgulama (`:40444`)
- **Web UI** — Pico CSS ile embedded dashboard
- **Analyze** — Keyword extraction, TLD istatistikleri, edu breakdown, wordlist export
- **Google Drive** — Apps Script webhook + queue mode ile Drive yedekleme (günde 100 dosya)
- **Download** — İsteğe bağlı metadata extraction (SHA256, text, image, PDF)
- **CLI** — `list`, `search`, `stats`, `analyze` komutları

## Gereksinimler

- Go 1.25
- systemd (user service)
- BadgerDB (embedded, harici bağımlılık yok)

## Kurulum

```bash
git clone https://github.com/bedri/open-directory-crawler.git
cd open-directory-crawler

# API keys (opsiyonel)
cp .env.example .env

# Build & deploy
./build.sh
```

## Kullanım

### Agent (sürekli crawl)

```bash
# Servis olarak başlat
systemctl --user start odk-agent

# Durum kontrol
systemctl --user status odk-agent

# Log takibi
tail -f odk-agent.log
```

### CLI

```bash
# İstatistikler
./odk stats

# Dosyaları listele
./odk list --json
./odk list --cat video --min-size 1048576
./odk list --export files.json

# Ara
./odk search --q "linux"

# Analiz
./odk analyze
./odk analyze --wordlist keywords.txt
./odk analyze --export report.json

# API sunucusu (standalone)
./odk api
```

### API Endpoints

```
GET  /stats              Temel istatistikler
GET  /stats/analysis     Full analiz raporu
GET  /stats/keywords     En çok kullanılan keywordler (?limit=100)
GET  /stats/tlds         TLD dağılımı
GET  /stats/edu          Eğitim siteleri kırılımı
GET  /stats/domains      En çok dosya barındıran domainler
GET  /wordlist           Keyword wordlist (text/plain)
GET  /dirs               Dizin listesi (?limit=N&offset=N)
GET  /dirs/:id           Dizin detay
GET  /dirs/:id/files     Dizindeki dosyalar
GET  /search?q=          Dosya ara (?q=&cat=)
GET  /files              Dosyalar (?cat=&ext=&limit=)
```

## Google Drive Entegrasyonu

1. `scripts/gdrive-webhook.gs`'yi Apps Script'e deploy et
2. Script Properties'e `GDRIVE_FOLDER` = Drive klasör ID'si
3. Queue oluştur: `./odk list --export odk-queue.json --cat video,audio,document,image --min-size 1048576`
4. Drive'a yükle, script timer ile otomatik işler

Günlük limit: 100 dosya (Apps Script quota).

## Mimari

```
┌─────────────────┐     ┌──────────────┐     ┌────────────────┐
│   Agent         │────▶│  odk.db      │────▶│  API (:40444)  │
│  (systemd user) │     │  (writer)    │     │  + Web UI      │
└─────────────────┘     └──────┬───────┘     └────────────────┘
                               │ sync (backup/load)
                      ┌───────▼────────┐     ┌────────────────┐
                      │ odk-reader.db  │◀────│  CLI (list/    │
                      │  (reader)      │     │   search/stats)│
                      └────────────────┘     └────────────────┘

Agent cycle:
  Crawl pending dirs → analysis → sync to reader → reset all → repeat
```

## Build

```bash
./build.sh   # test → build → restart agent
```

Test: `go test ./...`

## Proje Yapısı

```
├── cmd/              # CLI komutları (agent, api, list, search, stats, analyze)
├── internal/
│   ├── analysis/     # Keyword, TLD, istatistik analizi
│   ├── classify/     # Dosya kategorizasyonu
│   ├── crawler/      # HTTP/FTP crawl, download, metadata, GDrive
│   ├── discover/     # Open directory keşfi (arama motorları)
│   ├── envutil/      # .env / environment helper
│   ├── models/       # Veri tipleri
│   ├── parser/       # HTML dizin listesi ayrıştırıcı
│   ├── storage/      # BadgerDB depolama
│   └── webui/        # Embedded web arayüzü
├── scripts/          # Apps Script (GDrive entegrasyonu)
├── build.sh          # Build/deploy scripti
└── discover_dirs.txt # Keşfedilen/open directory URL listesi
```

## Lisans

MIT
