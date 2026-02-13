import logging
from pathlib import Path
from rich.logging import RichHandler
from orbit.exceptions import OrbitError
from orbit.common import get_application_directory

_logging_configured: bool = False

LOG_FILENAME = "orbit.log"

# Re-export the levels
DEBUG = logging.DEBUG
INFO = logging.INFO
WARNING = logging.WARNING
ERROR = logging.ERROR
CRITICAL = logging.CRITICAL

class ConsoleFormatter(logging.Formatter):
    """Different format based on log level."""
    
    RICH_MARKUP = {
        logging.DEBUG: "[green]{}[/green]",
        logging.INFO: "[blue]{}[/blue]",
        logging.WARNING: "[yellow]{}[/yellow]",
        logging.ERROR: "[red]{}[/red]",
        logging.CRITICAL: "[bold red]{}[/bold red]"
    }
    
    def __init__(self, level, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self._levelno = level
        
            
    def _apply_rich_markup(self, message, level):
        return self.RICH_MARKUP.get(level, "{}").format(message)
        
    
    def format(self, record):
        fmt = (
            "%(name)s: %(message)s" 
            if self._levelno == logging.DEBUG 
            else "%(message)s"
        )
        formatter = logging.Formatter(fmt)
        formatted_record = formatter.format(record)
        
        colored_output = self._apply_rich_markup(formatted_record, record.levelno)
        
        return colored_output


def setup_logging(level: int = logging.INFO, enable_file_logging: bool = False, reset: bool = False) -> None:
    """Set up logging for the application.
    
    :param level: The logging level (e.g. logging.DEBUG, logging.INFO).
    :param enable_file_logging: If True, also log to a file named "orbit.log" in the application directory
    :param reset: If True, reset existing logging configuration before setting up new configuration. Otherwise, an exception is raised
    :raise OrbitError: If logging is already configured and reset is False.
    """
    global _logging_configured
    
    if _logging_configured:
        if not reset:
            raise OrbitError("Unauthorised attempt to set logging anew.")
        else:
            reset_logging()
    
    console_handler = RichHandler(
        markup = True,
        show_time = level <= logging.INFO,
        show_path = level == logging.DEBUG,
        show_level = level < logging.INFO,
        omit_repeated_times = False,
        rich_tracebacks = True
    )
    console_handler.setLevel(level)
    console_handler.setFormatter(ConsoleFormatter(level))

    logging.basicConfig(
        level = logging.DEBUG,
        datefmt = "%Y-%m-%d %H:%M:%S",
        handlers = [console_handler]
    )
    
    if enable_file_logging:
        enable_file_logging()
        
    _logging_configured = True
    
def get_file_log_path() -> Path:
    return get_application_directory() / LOG_FILENAME
    
def enable_file_logging():
    global _logging_configured
    
    if not _logging_configured:
        raise OrbitError("Logging is not yet configured, cannot enable file logging.")
    
    root = logging.getLogger()
    if any(isinstance(h, logging.FileHandler) for h in root.handlers):
        raise OrbitError("File logging is already enabled.")
       
    debug_file_handler = logging.FileHandler(get_file_log_path())
    debug_file_handler.setLevel(logging.DEBUG)
    debug_file_handler.setFormatter(logging.Formatter(
        "%(asctime)s: %(name)s | %(levelname)-8s: %(message)s"
    ))
    root.addHandler(debug_file_handler)
    
    
def reset_logging():
    """Reset logging configuration to default state."""
    global _logging_configured
    
    root = logging.getLogger()
    for handler in root.handlers:
        root.removeHandler(handler)
        handler.close()
        
    root.setLevel(logging.WARNING)
    
    _logging_configured = False
    

def get_logger(name: str | None = None) -> logging.Logger:
    """Get a logger for the given name.
    
    This is a thin wrapper around logging.getLogger to ensure that the RichHandler is used consistently across the application.
    """
    return logging.getLogger(name)
