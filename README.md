# FoundationDB: транзакции и отказы

**Проектное задание:** [P09-A. FoundationDB: транзакции и отказы](https://distsys.ru/osukhoroslov/nis2/src/branch/main/project-tasks/p09-a-foundationdb-transactions.md)

**Участники:**

- Сабгир София
- Дервис Максим
- Широковских Александр

## Сокращенный запуск

Требования: Docker с Compose v2, 8 ГБ памяти для Docker, свободная подсеть `10.89.9.0/24`.
Кластер собирается из готового образа `foundationdb/foundationdb:8.0.0` (amd64 и arm64),
сборка FoundationDB из исходников не нужна.

```text
scripts/cluster-up.sh             # 5 процессов fdbserver, режим double ssd
scripts/roles.sh                  # какие роли на каком процессе
scripts/check-kill.sh fdb5 100    # 100 записей, kill -9 процесса, проверка сохранности
make image                        # образ p09-loadgen: генератор нагрузки и сводка
scripts/load.sh hot100 -hot 100 -duration 20s   # журнал в results/runs/hot100.jsonl и сводка
scripts/cluster-down.sh           # остановить кластер и удалить данные
make check                        # gofmt, vet, тесты, линтер, сборка образа
```

Генератор и формат журнала описаны в [docs/DESIGN.md](docs/DESIGN.md).

Технические проверки первой точки с ожидаемыми и полученными результатами описаны в
[nis/checkpoints/01.md](nis/checkpoints/01.md).

## Навигация

- [Описание проекта](nis/project.md)
- [План эксперимента](nis/experiment-plan.md)
- [Контрольная точка 1](nis/checkpoints/01.md)
- Генератор нагрузки: [cmd/loadgen](cmd/loadgen), [internal/](internal/), спецификация [docs/DESIGN.md](docs/DESIGN.md)
- Тесты: `*_test.go` в [internal/](internal/), запуск `make test`
- Кластер: [cluster/docker-compose.yml](cluster/docker-compose.yml)
- Скрипты кластера и проверок: [scripts/](scripts/)
- Результаты проверок: [results/checks/](results/checks/)
