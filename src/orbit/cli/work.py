"""`orbit work` command group"""

from typing import Annotated, List

import typer

app = typer.Typer()

@app.command()
def new(title: str = typer.Argument(help="Title of the new work entry.", metavar="TITLE")):
    """Create a new work entry."""
    pass

@app.command()
def list(
    tag_filter: List[str] = typer.Option(
        None, "--tag", "-t",
        help="Filter work entries to those tagged with the given tag (e.g., 'project:alpha')."
    )
):
    """List work entries, optionally filtered by tag."""
    # TODO: Allow multiple tag filters
    pass

@app.command()
def show(
    work_id: str = typer.Argument(help="ID of the work entry to show.", metavar="WORK_ID")
):
    """Show details of a work entry."""
    pass

@app.command()
def status():
    """Show the status of the currently active work entry, if any."""
    pass

@app.command()
def close(
    work_id: str = typer.Option(None, "--work-id", "-w", help="ID of the work entry to close. If None, the currently active work entry will be closed."),
):
    """Close a work entry, and mark it as completed."""
    pass

@app.command()
def abandon(
    reason: str = typer.Argument(help="Reason for abandoning the work entry.", metavar="REASON"),
    work_id: str = typer.Option(None, "--work-id", "-w", help="ID of the work entry to abandon. If None, the currently active work entry will be abandoned."),    
):
    """Close a work entry, and mark it as abandoned."""
    pass

@app.command()
def select(
    work_id: str = typer.Argument(help="ID of the work entry to select.", metavar="WORK_ID")
):
    """Select a work entry to be the default for subsequent commands."""
    pass

@app.command()
def forget():
    """Forget the currently selected work entry, if any."""
    pass

@app.command()
def show_selected():
    """Show the currently selected work entry, if any."""
    pass
