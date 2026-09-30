from pathlib import Path
from zipfile import ZIP_DEFLATED, ZipFile


# main собирает текущие исходники и документацию без рабочих настроек и результатов сборки.
def main():
    root = Path(__file__).resolve().parents[1]
    files = [
        root / name for name in (
            "README.md", "Отчёт.md", "go.mod", "go.sum", "Makefile",
            "Dockerfile", "compose.yaml", ".env.example", ".gitignore", ".dockerignore",
        )
    ]
    for directory in ("cmd", "internal", "migrations", "web", "tests", "scripts"):
        files.extend(
            path for path in (root / directory).rglob("*")
            if path.is_file() and "__pycache__" not in path.parts
        )

    destination = root / "dist" / "AccountingForExpenses.zip"
    destination.parent.mkdir(exist_ok=True)
    with ZipFile(destination, "w", compression=ZIP_DEFLATED) as archive:
        for path in sorted(files):
            archive.write(path, Path("AccountingForExpenses") / path.relative_to(root))
    print(f"Архив создан: {destination} ({len(files)} файлов)")


if __name__ == "__main__":
    main()
