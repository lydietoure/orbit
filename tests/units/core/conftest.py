import pytest
from pathlib import Path

@pytest.fixture(autouse=True)
def application_directory(mocker, tmp_path):
    """Fixture to set up a temporary application directory for testing."""
    app_dir = tmp_path / "orbit"
    app_dir.mkdir(parents=True, exist_ok=True)
    
    # Patch in both modules where get_application_directory is used
    mocker.patch(
        "orbit.core.config.get_application_directory",
        return_value=app_dir
    )
    mocker.patch(
        "orbit.core.db.get_application_directory",
        return_value=app_dir
    )
    
    return app_dir
    
    
