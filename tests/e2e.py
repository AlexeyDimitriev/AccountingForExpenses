import argparse
import concurrent.futures
import json
import os
from pathlib import Path
import secrets
import shlex
import signal
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request


class ComposeEnvironment:
    # __init__ изолирует имена ресурсов и настройки от рабочего окружения пользователя.
    def __init__(self, compose, env_file):
        self.root = Path(__file__).resolve().parents[1]
        self.project = "expenses-e2e-" + secrets.token_hex(6)
        self.command = shlex.split(compose) + [
            "--file", str(self.root / "compose.yaml"),
            "--env-file", str(env_file), "--project-name", self.project,
        ]
        self.environment = os.environ.copy()
        for line in (self.root / ".env.example").read_text().splitlines():
            if line.strip() and not line.lstrip().startswith("#") and "=" in line:
                self.environment.pop(line.split("=", 1)[0].strip(), None)
        for name in list(self.environment):
            if name.startswith("COMPOSE_"):
                self.environment.pop(name)
        self.environment.update(
            APP_VERSION=self.project,
            APP_PORT="0",
            APP_BIND_ADDRESS="127.0.0.1",
            POSTGRES_USER="expenses",
            POSTGRES_DB="expenses",
            POSTGRES_PASSWORD=secrets.token_hex(24),
            DATABASE_URL="host=db port=5432 sslmode=disable",
            COMPOSE_BAKE="false",
        )

    # run выполняет команду Compose только для отдельного тестового проекта.
    def run(self, *arguments, timeout=180):
        result = subprocess.run(
            self.command + list(arguments), cwd=self.root,
            env=self.environment, capture_output=True, text=True, timeout=timeout,
        )
        if result.returncode:
            raise RuntimeError(
                f"Compose {' '.join(arguments)} завершился с ошибкой:\n"
                + result.stdout + result.stderr
            )
        return result.stdout

    # start запускает две копии сервера после готовности базы и выполнения миграций.
    def start(self):
        self.run("up", "--detach", "--no-build", "--scale", "app=2", "--wait", "--wait-timeout", "90")
        return self.addresses()

    # addresses возвращает опубликованные адреса обоих экземпляров сервера.
    def addresses(self):
        return [
            "http://" + self.run("port", "--index", str(index), "app", "8080").strip()
            for index in (1, 2)
        ]

    # cleanup удаляет только контейнеры, сеть и том созданного тестового проекта.
    def cleanup(self):
        self.run("down", "--volumes", "--remove-orphans", timeout=60)


# request проверяет HTTP-статус и читает тело JSON или статического файла.
def request(address, method, path, payload=None, status=200, json_response=True):
    body = None if payload is None else json.dumps(payload).encode()
    query = urllib.request.Request(
        address + path, data=body, method=method,
        headers={"Content-Type": "application/json"},
    )
    try:
        response = urllib.request.urlopen(query, timeout=15)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        content = response.read()
        expected = (status,) if isinstance(status, int) else status
        if response.status not in expected:
            raise AssertionError(f"{method} {path}: {response.status}, ожидался {status}; {content!r}")
        if response.status == 204:
            assert not content, "Ответ 204 не должен содержать тело"
            return None
        if not json_response:
            return content.decode()
        assert response.headers.get_content_type() == "application/json"
        result = json.loads(content)
        if response.status >= 400:
            assert result["error"]["code"] and result["error"]["message"]
        return result


# wait_ready ждёт восстановления подключения к БД после перезапуска сервиса.
def wait_ready(address):
    deadline = time.monotonic() + 40
    last_error = None
    while time.monotonic() < deadline:
        try:
            request(address, "GET", "/readyz")
            return
        except (OSError, AssertionError) as error:
            last_error = error
            time.sleep(0.2)
    raise AssertionError(f"Сервер {address} не готов: {last_error}")


# check_frontend проверяет готовность обоих процессов и встроенные ресурсы интерфейса.
def check_frontend(environment):
    first, second = environment.addresses()
    assert first != second, "Ожидались два разных опубликованных адреса"
    for address in (first, second):
        assert request(address, "GET", "/healthz") == {"status": "ok"}
        assert request(address, "GET", "/readyz") == {"status": "ok"}
        assert "Личный бюджет" in request(address, "GET", "/", json_response=False)
        assert "initialize" in request(address, "GET", "/assets/app.mjs", json_response=False)
        assert "parseAmount" in request(address, "GET", "/assets/money.mjs", json_response=False)
        request(address, "GET", "/missing", status=404)
    assert environment.run("exec", "-T", "app", "id", "-u").strip() == "65532"


# check_crud проверяет операции и фильтры, чередуя обращения к двум экземплярам сервера.
def check_crud(environment):
    first, second = environment.addresses()
    category = request(first, "POST", "/api/categories", {"name": "Продукты"}, 201)
    other = request(second, "POST", "/api/categories", {"name": "Транспорт"}, 201)
    category_path = f"/api/categories/{category['id']}"
    assert request(second, "GET", category_path) == category
    renamed = request(second, "PUT", category_path, {"name": "Покупки"})
    assert request(first, "GET", category_path) == renamed
    assert len(request(first, "GET", "/api/categories")) == 2
    expenses = []
    for address, category_id, amount, date in [
        (first, category["id"], 101, "2026-09-28"),
        (second, category["id"], 202, "2026-09-29"),
        (first, other["id"], 303, "2026-09-29"),
    ]:
        expenses.append(request(address, "POST", "/api/expenses", {
            "category_id": category_id, "amount_kopecks": amount,
            "date": date, "comment": "Сквозная проверка",
        }, 201))
    filters = [
        ("", 3, 606),
        (f"category_id={category['id']}", 2, 303),
        ("date_from=2026-09-29", 2, 505),
        ("date_to=2026-09-28", 1, 101),
        (f"category_id={category['id']}&date_from=2026-09-29&date_to=2026-09-29", 1, 202),
        ("date_from=2026-10-01", 0, 0),
    ]
    for query, count, total in filters:
        result = request(second, "GET", "/api/expenses?" + query)
        assert len(result["expenses"]) == count and result["total_kopecks"] == total
    expense_path = f"/api/expenses/{expenses[0]['id']}"
    assert request(second, "GET", expense_path) == expenses[0]
    changed = request(second, "PUT", expense_path, {
        "category_id": other["id"], "amount_kopecks": 999, "date": "2026-09-30",
    })
    assert request(first, "GET", expense_path) == changed
    request(first, "DELETE", category_path, status=409)
    for invalid in [
        {"category_id": category["id"], "amount_kopecks": 0, "date": "2026-09-30"},
        {"category_id": category["id"], "amount_kopecks": 1, "date": "2026-02-30"},
    ]:
        request(first, "POST", "/api/expenses", invalid, 400)
    request(second, "GET", "/api/expenses?date_from=2026-10-01&date_to=2026-09-01", status=400)
    for expense in expenses:
        path = f"/api/expenses/{expense['id']}"
        request(first, "DELETE", path, status=204)
        request(second, "GET", path, status=404)
    for item in (category, other):
        path = f"/api/categories/{item['id']}"
        request(second, "DELETE", path, status=204)
        request(first, "GET", path, status=404)
    assert request(first, "GET", "/api/expenses") == {"expenses": [], "total_kopecks": 0}


# check_parallel_writes проверяет общую БД при одновременных записях через две копии бэкенда.
def check_parallel_writes(environment):
    addresses = environment.addresses()
    category = request(addresses[0], "POST", "/api/categories", {"name": "Параллельные запросы"}, 201)

    # create_expense создаёт запись через один из двух экземпляров сервера.
    def create_expense(index):
        return request(addresses[index % 2], "POST", "/api/expenses", {
            "category_id": category["id"], "amount_kopecks": 1, "date": "2026-09-30",
        }, 201)

    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as executor:
        expenses = list(executor.map(create_expense, range(10)))
    assert len({expense["id"] for expense in expenses}) == 10
    for address in addresses:
        result = request(address, "GET", f"/api/expenses?category_id={category['id']}")
        assert len(result["expenses"]) == 10 and result["total_kopecks"] == 10
    for expense in expenses:
        request(addresses[0], "DELETE", f"/api/expenses/{expense['id']}", status=204)
    request(addresses[1], "DELETE", f"/api/categories/{category['id']}", status=204)


# check_persistence проверяет повторные миграции, перезапуск и пересоздание контейнеров без потери данных.
def check_persistence(environment):
    first, second = environment.addresses()
    category = request(first, "POST", "/api/categories", {"name": "Сохранность данных"}, 201)
    expense = request(second, "POST", "/api/expenses", {
        "category_id": category["id"], "amount_kopecks": 12345, "date": "2026-09-30",
    }, 201)
    path = f"/api/expenses/{expense['id']}"
    environment.run("run", "--rm", "--no-deps", "migrate")
    assert request(first, "GET", path) == expense
    environment.run("restart", "app")
    for address in environment.addresses():
        wait_ready(address)
        assert request(address, "GET", path) == expense
    environment.run("down")
    for address in environment.start():
        assert request(address, "GET", path) == expense


# check_database_recovery проверяет независимость liveness и восстановление readiness после сбоя БД.
def check_database_recovery(environment):
    addresses = environment.addresses()
    before = request(addresses[0], "GET", "/api/expenses")
    environment.run("stop", "db")
    for address in addresses:
        request(address, "GET", "/healthz")
        request(address, "GET", "/readyz", status=503)
        request(address, "GET", "/api/expenses", status=(500, 504))
    environment.run("up", "--detach", "--no-deps", "--wait", "--wait-timeout", "60", "db")
    for address in addresses:
        wait_ready(address)
        assert request(address, "GET", "/api/expenses") == before


# interrupt переводит SIGTERM в обычное завершение с очисткой тестовых ресурсов.
def interrupt(signum, frame):
    raise KeyboardInterrupt


# main запускает сценарии и удаляет временное окружение независимо от результата проверок.
def main():
    parser = argparse.ArgumentParser(description="Сквозная проверка приложения в отдельном Docker Compose проекте")
    parser.add_argument("--compose", default="docker compose", help="команда Docker Compose")
    arguments = parser.parse_args()
    signal.signal(signal.SIGTERM, interrupt)
    with tempfile.TemporaryDirectory(prefix="expenses-e2e-") as directory:
        env_file = Path(directory) / "test.env"
        env_file.touch()
        environment = ComposeEnvironment(arguments.compose, env_file)
        status = 0
        try:
            print(f"Тестовый проект: {environment.project}", flush=True)
            environment.run("build", "app", timeout=600)
            environment.start()
            for name, check in [
                ("Готовность и фронтенд", check_frontend),
                ("CRUD и фильтры через два экземпляра", check_crud),
                ("Одновременные записи", check_parallel_writes),
                ("Миграции и сохранность данных", check_persistence),
                ("Остановка и восстановление БД", check_database_recovery),
            ]:
                print(f"Проверка: {name}", flush=True)
                check(environment)
                print("  OK", flush=True)
        except (Exception, KeyboardInterrupt) as error:
            status = 1
            print(f"Проверка завершилась с ошибкой: {error!r}", file=sys.stderr)
            try:
                print(environment.run("logs", "--no-color", "--tail", "60"), file=sys.stderr)
            except Exception as log_error:
                print(f"Не удалось получить логи: {log_error}", file=sys.stderr)
        finally:
            try:
                environment.cleanup()
                print("Тестовые контейнеры, сеть и том удалены.", flush=True)
            except Exception as cleanup_error:
                status = 1
                print(f"Не удалось очистить проект {environment.project}: {cleanup_error}", file=sys.stderr)
        return status


if __name__ == "__main__":
    sys.exit(main())
