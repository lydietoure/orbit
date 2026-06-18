"""`orbit log` command."""

import typer
from orbit.cli.common import WorkIdOption

app = typer.Typer()

@app.command()
def new(
    message: str = typer.Argument(..., help="The log message to append."),
    work_id: str = typer.Option(None, "--work-id", "-w", help="ID of the work entry to append the log to. If None, the default selected work entry will be used."),
):
    """Append a log to a work entry."""
    pass

@app.command()
def list(
    work_id: str = WorkIdOption,
    since_filter: str = typer.Option(None, "--since", help="Filter logs to those created since the given time (e.g., '2024-01-01T00:00:00Z', '2w')."),
    all: bool = typer.Option(False, help="Show log entries across all work entries")
):
    """List logs for a work entry."""
    pass
