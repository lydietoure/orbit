
import typer


from orbit import (
    __APPLICATION_NAME__ as APP_NAME,
    __version__ as APP_VERSION,
)

app = typer.Typer(
    name = APP_NAME,
    rich_help_panel = "Orbit High-Level Commands",
)

def __version_callback(value: bool) -> None:
    if value:
        typer.echo(f"{APP_NAME} version: {APP_VERSION}")
        raise typer.Exit()
    
app.callback()
def main_callback(
    version: bool = typer.Option(False, "--version", "-V", help="Show the application's version and exit.", callback=__version_callback, is_eager=True)
):
    """Your developer universe, mapped and in motion.
    
    Orbit is a personal work tracker that unifies your git branches, pull requests,
    notes, and learnings into a single queryable graph. Local-first. CLI-first. LLM-ready.
    """
    pass

#region Lifecycle
@app.command()
def initialize():
    """Initialise the application by creating the necessary configuration files and directories."""
    print("Initializing the application...")

@app.command()
def status():
    """Check the status of your current work and see an overview of your projects."""
    print("Checking status...")
    
@app.command()
def summary():
    """Generate a summary of recent work"""
    print("Generating summary...")

#endregion

#region subcommand groups

#endregion
