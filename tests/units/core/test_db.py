"""Unit tests for orbit.core.db"""

import pytest
from sqlite3 import OperationalError
from contextlib import ExitStack as does_not_raise
from pytest_mock import MockerFixture, MockType
from unittest.mock import MagicMock
from pathlib import Path

from orbit.core.db import (
    get_database,
    initialise_database,
    DatabaseTables,

)

from tests.units.common import FixtureFunctionParams

MOCKED_DATABASE = MagicMock()


@pytest.fixture
def mocked_database(mocker: MockerFixture) -> MockType:
    """Fixture to mock the database connection."""
    db = MagicMock()
    db.executescript = lambda val: None  # No-op for executescript
    return mocker.patch("orbit.core.db.Database", return_value=db)

@pytest.fixture
def patched_database(mocker: MockerFixture, request: pytest.FixtureRequest) -> MockType:
    """Fixture to patch the Database class in orbit.core.db."""
    specs: FixtureFunctionParams = request.param
    
    return mocker.patch(
        "orbit.core.db.Database", 
        return_value=specs.return_value,
        side_effect=specs.side_effect
    )

@pytest.mark.parametrize(
    "expectation, path_exists, not_exist_ok, patched_database, expected_output",
    [
        (pytest.raises(FileNotFoundError), False, False, FixtureFunctionParams(), None),
        (does_not_raise(), False, True, FixtureFunctionParams(MOCKED_DATABASE), MOCKED_DATABASE),
        (pytest.raises(OperationalError), True, True, FixtureFunctionParams(None, OperationalError()), None),
    ],
    ids=[
        "missing-file-error",
        "missing-file-ok",
        "invalid-db-file-error",
    ],
    indirect=["patched_database"]
)
def test_get_database(
    mocker: MockerFixture, application_directory, patched_database,
    expectation, path_exists, not_exist_ok, expected_output
):
    """Test the core.db.get_database function."""
    
    # Mock the database path's existence and the database method
    mocker.patch.object(Path, "exists", return_value=path_exists)
    MOCKED_DATABASE.execute = lambda val: None  # No-op 
    

    with expectation:
        db = get_database(not_exist_ok=not_exist_ok)
        assert db == expected_output


def test_initialise_database(application_directory):
    
    db = initialise_database()
    assert db is not None
    
    for table in DatabaseTables:
        assert table.value in db.table_names(), f"Table {table.value} not found"
