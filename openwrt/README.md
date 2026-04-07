# Unbound-WRT — Пакет обхода DPI для OpenWrt

Прозрачный обход DPI/цензуры на уровне роутера для всей локальной сети. Нулевая настройка на клиентских устройствах.

## Структура директорий

```
openwrt/
├── README.md                          # Этот файл
├── unbound-wrt/                       # Основной пакет nfqws
│   ├── Makefile                       # Makefile пакета OpenWrt
│   └── files/
│       ├── etc/config/unbound         # Конфигурация UCI по умолчанию
│       ├── etc/init.d/unbound         # init-скрипт procd
│       └── etc/nftables.d/90-unbound-wrt.nft  # Правила fw4/nftables
│
└── luci-app-unbound/                  # Веб-интерфейс LuCI
    ├── Makefile                       # Makefile пакета LuCI
    └── luasrc/
        ├── controller/unbound.lua     # Регистрация в меню + API статуса
        └── model/cbi/unbound/unbound.lua  # Модель конфигурации CBI
```

## Архитектура

```
Клиенты LAN (без настройки)
    │ br-lan (TCP 80/443)
    ▼
fw4 / nftables (90-unbound-wrt.nft)
  - Перехватывает forward-трафик с br-lan
  - Исключения: RFC1918, широковещательные, сам роутер
  - Отправляет подходящие пакеты в NFQUEUE 200
    ▼
Демон nfqws (управляется procd)
  - Получает пакеты через NFQUEUE 200
  - Применяет стратегию: disorder, split-tls, fake и т.д.
  - Повторно вводит изменённые пакеты в стек ядра
    ▼
WAN / Аплинк
```

## Сборка

### Требования

- OpenWrt SDK, соответствующий вашей целевой версии (22.03 или 23.05)
- `libnetfilter-queue` и `libnetfilter-conntrack` в фидах SDK

### Шаги

1. **Клонируйте OpenWrt SDK:**
   ```bash
   wget https://downloads.openwrt.org/releases/23.05.0/targets/<arch>/<target>/openwrt-sdk-23.05.0-<arch>-<target>.Linux-x86_64.tar.xz
   tar xf openwrt-sdk-*.tar.xz
   cd openwrt-sdk-*
   ```

2. **Скопируйте пакеты в SDK:**
   ```bash
   cp -r /path/to/unbound-wrt package/
   cp -r /path/to/luci-app-unbound package/
   ```

3. **Обновите фиды:**
   ```bash
   ./scripts/feeds update -a
   ./scripts/feeds install -a
   ```

4. **Выберите пакеты в menuconfig:**
   ```bash
   make menuconfig
   ```
   - `Network > Web Servers/Proxies > nfqws-unbound` → установите в `M`
   - `LuCI > 3. Applications > luci-app-unbound` → установите в `M`

5. **Скомпилируйте:**
   ```bash
   make package/nfqws-unbound/compile V=s
   make package/luci-app-unbound/compile V=s
   ```

6. **Результат — файлы `.ipk`:**
   ```
   bin/packages/<arch>/base/nfqws-unbound_*.ipk
   bin/packages/<arch>/luci/luci-app-unbound_*.ipk
   ```

## Установка

```bash
# Передать на роутер
scp bin/packages/*/base/nfqws-unbound_*.ipk root@192.168.1.1:/tmp/
scp bin/packages/*/luci/luci-app-unbound_*.ipk root@192.168.1.1:/tmp/

# Установить на роутере
ssh root@192.168.1.1
opkg install /tmp/nfqws-unbound_*.ipk
opkg install /tmp/luci-app-unbound_*.ipk

# Включить и запустить
/etc/init.d/unbound enable
/etc/init.d/unbound start
```

## Настройка

### Через веб-интерфейс LuCI

Перейдите в **Сервисы > Unbound-WRT** в LuCI:

| Настройка | Описание |
|-----------|----------|
| **Включить** | Главный переключатель двигателя обхода DPI |
| **Стратегия обхода** | Стратегия изменения пакетов (см. ниже) |
| **Исключённые домены** | Домены, обходящие nfqws (по одному на строку) |
| **Исключённые IP** | Диапазоны IP/CIDR, обходящие nfqws (по одному на строку) |

### Через CLI (UCI)

```bash
uci set unbound.@general[0].enabled='1'
uci set unbound.@general[0].strategy='multidisorder'
uci set unbound.@general[0].exclude_ips='192.168.1.100 10.0.0.0/8'
uci commit unbound
/etc/init.d/unbound restart
```

## Стратегии обхода

| Стратегия | Описание | Лучше всего для |
|-----------|----------|-----------------|
| **Multidisorder** | Нарушает порядок сегментов пакетов | Общее назначение |
| **Split TLS** | Разбивает TLS ClientHello | Блокировка SNI на основе TLS |
| **Fake Ping** | Вводит поддельные пакеты с низким TTL | Агрессивный DPI |
| **Disorder + Fake** | Комбинирует disorder + поддельные пакеты | Максимальное уклонение |

## Установленные файлы

| Путь | Назначение |
|------|-----------|
| `/usr/bin/nfqws` | Демон NFQUEUE (кросс-компилированный C-бинарник) |
| `/etc/init.d/unbound` | Скрипт управления сервисом procd |
| `/etc/config/unbound` | Файл конфигурации UCI |
| `/etc/nftables.d/90-unbound-wrt.nft` | Правила перехвата nftables |

## Диагностика

```bash
# Проверка статуса сервиса
/etc/init.d/unbound status
logread | grep nfqws

# Проверка правил nftables
nft list chain inet fw4 unbound_wrt_forward
nft list chain inet fw4 unbound_wrt_lan_check

# Проверка получения пакетов NFQUEUE
nft list ruleset | grep queue

# Тест подключения с клиента LAN
tcpdump -i br-lan tcp port 443
```

## Лицензия

GPL-3.0-only
