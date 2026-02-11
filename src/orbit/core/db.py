"""Database operations."""

import shutil
from importlib.resources import files
from pathlib import Path

from sqlite_utils import Database

from orbit.core.config import get_application_directory


def get_database_path() -> Path:
    """Get the path to the database file."""
    app_dir = get_application_directory()
    db_path = app_dir / "orbit.db"
    return db_path


def get_database() -> Database:
    """Get the database connection."""
    db = Database(get_database_path())

    db.execute("PRAGMA foreign_keys = ON;")
    db.execute("PRAGMA journal_mode = WAL;")
    return db


def _load_schema() -> str:
    """Load the schema SQL from package resources."""
    return files("orbit.core").joinpath("schema.sql").read_text()


def initialise_database() -> None:
    """Initialize the database schema. Idempotent."""
    db = get_database()
    schema = _load_schema()
    db.executescript(schema)


def reset_database() -> None:
    """Delete and reinitialize the database.

    Removes orbit.db and any WAL/SHM files, then recreates the schema.
    """
    db_path = get_database_path()

    # Delete database and associated WAL files
    for suffix in ("", "-wal", "-shm"):
        path = db_path.parent / f"{db_path.name}{suffix}"
        if path.exists():
            path.unlink()

    # Reinitialize
    initialise_database()

