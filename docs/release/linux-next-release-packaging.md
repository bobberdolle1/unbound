# Экспериментальный Linux `amd64` target следующего релиза

## Релизная поверхность

Первый публичный Linux-артефакт следующего релиза — CLI-only `linux/amd64` в формате `unbound-v<VERSION>-linux-amd64.tar.gz`. Он будет явно помечен **EXPERIMENTAL**. Wails GUI не входит в этот архив: переносимая совместимость WebKit и пакетов дистрибутивов не доказана воспроизводимой матрицей. `linux/arm64`, AppImage, Flatpak, Snap, DEB и RPM не входят в эту поставку.

Это только инфраструктура следующего релиза. Она не меняет тег, assets или манифесты опубликованного v0.7.0 и не означает, что Linux-артефакт уже опубликован.

## Контракт архива

Архив содержит единственный корень `unbound-v<VERSION>-linux-amd64/`:

- CLI-бинарник `unbound` с внедрёнными `version`, `commit`, `dirty=false`, `channel=release`, `os=linux` и `arch=amd64`.
- `README.md`, `CHANGELOG.md`, `LICENSE`, `ZAPRET2_LICENSE.txt`, `ZAPRET_LICENSE.txt`, `ENGINE_PROVENANCE.json`, `ENGINE_ASSETS.sha256` и `BUNDLE_SHA256SUMS.txt`.
- `runtime/` с закрытым набором Linux runtime-ассетов: `nfqws2`, `ip2net`, `mdig`, применяемые Lua-скрипты, payload-файлы и списки.
- `scripts/` только с поддерживаемыми Linux CLI-лаунчерами.

`BUNDLE_SHA256SUMS.txt` покрывает каждый обычный payload-файл, кроме самого себя. CLI до выполнения команды проверяет соседний манифест и отклоняет отсутствующий, добавленный или изменённый payload. В частности, подмена или удаление упакованного `nfqws2` завершается до изменения packet path.

## Локальная сборка и проверка

На Linux `x86_64` локальный development-артефакт создаётся без публикации:

```bash
commit="$(git rev-parse HEAD)"
version="$(node -p "require('./wails.json').info.productVersion")"
./scripts/build/package_linux_release.sh \
  --version "$version" --expected-commit "$commit" --mode local
```

Release-координатор для будущего тега запускается только из чистого checkout и требует точный commit:

```bash
./scripts/release/local_release_linux.sh <VERSION> <EXPECTED_COMMIT>
```

Он выполняет `bash ./scripts/ci/linux.sh all`, собирает архив, проверяет identity, `BUNDLE_SHA256SUMS.txt`, содержимое, права, отсутствие ссылок/опасных путей, распакованный `--version`, `--version --json`, `--help`, `--list-profiles --json` и `--test`, затем пишет `release/linux-release-evidence.json`.

Архив создаётся с сортированными путями, нормализованными owner/group и timestamp commit, а gzip запускается без timestamp. Воспроизводимость относится к одинаковому commit и поддерживаемому toolchain; её нужно подтверждать на Linux release-host для каждого будущего тега.

## Runtime и границы поддержки

Поддерживаемый перехват использует `NFQUEUE` + `nfqws2` v1.0.5.1. Для packet interception требуется root, ядро с NFQUEUE и `nft`; `iptables` остаётся fallback там, где он поддержан runtime. Непривилегированные `--version`, `--help`, `--list-profiles --json` и `--test` не доказывают возможность создать NFQUEUE.

Нативная физическая приёмка следующего артефакта обязана запускаться из распакованного `tar.gz`: проверяются ownership NFQUEUE/firewall, `nft` primary path, cleanup при normal stop/SIGTERM/SIGINT, отсутствие stale state, повторный запуск и сохранение foreign firewall state. Отсутствие подходящего root Linux-host не превращает кросс-сборку в доказательство runtime.

Linux остаётся экспериментальным. Пакет не обещает GUI, поддержку каждого дистрибутива, все пакетные менеджеры или универсальную эффективность обхода для YouTube, Discord либо иного сервиса.
