"""Database operations."""

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
