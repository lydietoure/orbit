import platformdirs
from pathlib import Path
from orbit import __APPLICATION_NAME__ as APP_NAME

def get_application_directory() -> Path:
    """Get the application directory for orbit."""
    app_dir = platformdirs.user_config_dir(appname=APP_NAME)
    return Path(app_dir)
