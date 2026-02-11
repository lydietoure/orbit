"""Manage application lifecycle: build, teardown, recovery, migration"""

import shutil

from orbit.core.config import get_application_directory
    
def reset_application() -> None:
    """Delete the entire application directory (~/.orbit/).

    This removes all data: database, config, everything.
    """
    app_dir = get_application_directory()
    if app_dir.exists():
        shutil.rmtree(app_dir)
