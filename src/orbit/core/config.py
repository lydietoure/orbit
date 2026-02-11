"""Handles loading, saving, and managing the high-level application configuration for orbit."""

import yaml
import shutil
import platformdirs

from pathlib import Path
from pydantic import BaseModel, ConfigDict

from orbit import __APPLICATION_NAME__ as APP_NAME


class DefaultsConfig(BaseModel):
    """Default configuration values for orbit."""
    owner: str | None = None
    project: str | None = None
    
class PathsConfig(BaseModel):
    notes_root: str | None = None

class Configuration(BaseModel):
    """Configuration object for orbit."""
    model_config = ConfigDict(
        extra = "forbid"
    )
    
    defaults: DefaultsConfig = DefaultsConfig()
    paths: PathsConfig = PathsConfig()

def get_application_directory() -> Path:
    """Get the application directory for orbit."""
    app_dir = platformdirs.user_config_dir(appname=APP_NAME)
    return Path(app_dir)

def get_configuration_file_path() -> Path:
    """Get the path to the configuration file."""
    app_dir = get_application_directory()
    config_file_path = app_dir / "config.yaml"
    return config_file_path

def load_configuration(must_exist: bool = False) -> Configuration:
    """Load the configuration for orbit."""
    cfg_path = get_configuration_file_path()
    if not cfg_path.exists() and not must_exist:
        return Configuration()
    elif not cfg_path.exists() and must_exist:
        raise FileNotFoundError(f"Configuration file not found at {cfg_path}")
    
    # Load the configuration file    
    with open(cfg_path, "rb") as f:
        cfg_data = yaml.safe_load(f)
    return Configuration(**cfg_data)

def save_configuration(config: Configuration) -> None:
    """Save the configuration for orbit."""
    cfg_path = get_configuration_file_path()
    cfg_path.parent.mkdir(parents=True, exist_ok=True)
    
    model_dict = config.model_dump()
    with open(cfg_path, "w") as f:
        yaml.safe_dump(model_dict, f)
        
    return
    
    
def reset_application() -> None:
    """Delete the entire application directory (~/.orbit/).

    This removes all data: database, config, everything.
    """
    app_dir = get_application_directory()
    if app_dir.exists():
        shutil.rmtree(app_dir)
    
