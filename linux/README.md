# Unbound Linux — Демон обхода DPI/цензуры и плагин Decky

Высокопроизводительный инструмент обхода DPI/цензуры для Linux, написанный на Rust. Обёртка над бинарным файлом `nfqws` из проекта [zapret](https://github.com/bol-van/zapret) с автоматическим управлением правилами nftables.

## Архитектура

```
Пользовательский интерфейс
  CLI (sudo)              Плагин Decky (Game Mode)
  unbound-cli             Preact UI + Python backend
          \                   /
           v                 v
        unbound-cli демон (Rust, работает от root)
           /                \
      nftables            nfqws (zapret)
      правила             процесс
     (NFQUEUE)
```

## Компоненты

| Файл | Описание |
|------|----------|
| `src/main.rs` | Точка входа CLI с подкомандами `start`, `stop`, `status` + автоопределение `nfqws` |
| `src/nftables_mgr.rs` | Динамическое применение/удаление/очистка правил nftables с синтаксисом, совместимым с zapret |
| `src/nfqws.rs` | Менеджер процессов: запуск/остановка/мониторинг `nfqws` через PID-файл, плавное завершение от SIGTERM до SIGKILL |
| `src/daemon.rs` | Оркестратор жизненного цикла: применяет правила, запускает nfqws, ожидает SIGINT/SIGTERM, очищает всё |
| `src/config.rs` | Структура конфигурации демона с разбором портов |
| `src/error.rs` | Типизированный enum ошибок |

## Сборка

```bash
cargo build --release
# Бинарник: ../target/release/unbound-cli
```

## Использование (требуются права root)

```bash
# Запуск с настройками по умолчанию (queue 200, автоопределение интерфейса)
sudo unbound-cli start

# Запуск с указанием интерфейса и очереди
sudo unbound-cli start --iface eth0 --queue 200

# Проверка статуса
sudo unbound-cli status

# Остановка (автоматически очищает правила nftables)
sudo unbound-cli stop
```

## Правила nftables

При работе демон создаёт:

```
table inet unbound {
    chain post {
        type filter hook postrouting priority mangle;
        oifname "eth0" meta mark and 0x40000000 == 0 tcp dport {80,443} ct original packets 1-6 queue num 200 bypass
        oifname "eth0" meta mark and 0x40000000 == 0 udp dport {443} ct original packets 1-6 queue num 200 bypass
    }
    chain pre {
        type filter hook prerouting priority filter;
        iifname "eth0" tcp sport {80,443} ct reply packets 1-3 queue num 200 bypass
    }
}
```

Правила **автоматически удаляются** при завершении демона (SIGINT/SIGTERM/сбой), чтобы пользователь никогда не остался без интернета.

## Сервис systemd

```bash
sudo cp ../packaging/unbound.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now unbound.service
```

## Зависимости

- **Для работы:** `nftables`, `libnetfilter_queue`, `nfqws` (из zapret)
- **Для сборки:** `cargo`, `rust`

## Упаковка

См. `../packaging/` для:
- `PKGBUILD` — Arch Linux AUR
- `build-deb.sh` — Debian/Ubuntu
- `build-rpm.sh` — Fedora/RHEL
- `unbound.service` — файл сервиса systemd

## Лицензия

GPL-3.0
