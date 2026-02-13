"""Shared test utilities for unit tests."""

from dataclasses import dataclass
from typing import Any


@dataclass
class FixtureFunctionParams:
    """Specification for configuring a mock's return value or side effect."""
    return_value: Any = None
    side_effect: Exception | None = None
