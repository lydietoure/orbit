"""Common reusable code for CLI commands."""

import typer


def _resolve_work_id(work_id: str | None) -> str:
    """Resolve the work ID to use for a command, falling back to the default selected work entry if None."""
    # TODO: Implement validation steps and get the default work entry if exists, or raise an error
    return work_id
WorkIdOption = typer.Option(
    None,
    "--work-id", "-w",
    help="ID of the work entry to operate on. If None, the default selected work entry will be used."
)
