import json
import pathlib
import sqlite3

root = pathlib.Path(__file__).resolve().parents[3] / "backend/app/migrations"
result = {}
for database, directory in (("MAIN", "sqlite"), ("TELEMETRY", "sqlite_telemetry")):
    migrations = []
    for path in sorted((root / directory).glob("*.up.sql")):
        statements = []
        pending = ""
        for character in path.read_text():
            pending += character
            if character == ";" and sqlite3.complete_statement(pending):
                statements.append(pending.strip())
                pending = ""
        if pending.strip():
            statements.append(pending.strip())
        migrations.append({"name": path.name, "statements": statements})
    result[database] = migrations
print(json.dumps(result))
