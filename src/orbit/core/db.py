"""Database operations."""

from importlib.resources import files
from pathlib import Path
from enum import StrEnum
from sqlite_utils import Database

from orbit.core.config import get_application_directory

DATABASE_FILENAME = "orbit.db"

class DatabaseTables(StrEnum):
    """Names of the database tables."""
    
    WORK_ENTRIES = "work_entries"
    TAGS = "tags"
    ARTIFACTS = "artifacts"
    NOTES = "notes"
    LOG_ENTRIES = "log_entries"
    WORK_DAYS = "work_days"
    
    # Application state: singleton
    STATE = "state"
    
    # Join table for many-to-many relationship between WorkEntries and Tags
    WORK_ENTRY_TAGS = "work_entry_tags"
    

def get_database_path() -> Path:
    """Get the path to the database file."""
    app_dir = get_application_directory()
    db_path = app_dir / "orbit.db"
    return db_path


def get_database(not_exist_ok: bool = False) -> Database:
    """Get the database connection."""
    
    path = get_database_path()
    if not not_exist_ok and not path.exists():
        raise FileNotFoundError(f"Database file not found at {path}")
    
    # TODO: When adding logging, consider a tracer for database operations.
    db = Database(path)

    db.execute("PRAGMA foreign_keys = ON;")
    db.execute("PRAGMA journal_mode = WAL;")
    return db


def _load_schema() -> str:
    """Load the schema SQL from package resources."""
    return files("orbit.core").joinpath("schema.sql").read_text()


def initialise_database() -> Database:
    """Initialize the database schema. Idempotent."""
    # TODO: Catch sqlite3.OperationalError and raise custom exception
    db = get_database(not_exist_ok=True)
    schema = _load_schema()
    db.executescript(schema)
    
    return db

def clear_database() -> None:
    """Clear and delete the database file."""
    db_path = get_database_path()

    # Delete database and associated WAL files
    for suffix in ("", "-wal", "-shm"):
        path = db_path.parent / f"{db_path.name}{suffix}"
        if path.exists():
            path.unlink()
    

def reset_database() -> None:
    """Delete and reinitialize the database.

    Removes orbit.db and any WAL/SHM files, then recreates the schema.
    """
    clear_database()
    initialise_database()
