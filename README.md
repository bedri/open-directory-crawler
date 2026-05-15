# open-directory-crawler

Open directory crawler, indexer, and search engine.

## Features

- **Discover** — Google dork, Bing, DuckDuckGo ile open directory keşfi
- **Crawl** — Recursive dizin tarama, dosya kategorizasyonu
- **Agent** — Sürekli çalışan background crawler (systemd user service)
- **API** — REST API ile sorgulama (`/stats`, `/dirs`, `/files`, `/search`)
- **Web UI** — Gömülü dashboard ile görsel arayüz (`odk api`)

## Quick Start

```bash
./build.sh          # test + build + restart agent
odk api --cors      # web UI + API sunucusu
```

## Komutlar

| Komut | Açıklama |
|-------|----------|
| `odk agent` | Background crawler (systemd user service ile yönetilir) |
| `odk api` | REST API + Web UI sunucusu |
| `odk discover --all` | Open directory keşfet |
| `odk discover --import liste.txt` | URL listesi import et |
| `odk discover --check <url>` | URL open directory mi kontrol et |

## Service Management

```bash
systemctl --user start odk-agent
systemctl --user stop odk-agent
systemctl --user status odk-agent
journalctl --user -u odk-agent -f
```

## License

MIT
