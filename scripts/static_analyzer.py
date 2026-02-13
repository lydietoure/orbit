"""This is a series of static analysis checks on the code base. Run with `pytest scripts/static_analyzer.py`"""

import ast
from importlib.resources import files

def test_exceptions_does_not_import_logging():
    """Test that the orbit.exceptions module does not import the orbit.logging module, to avoid circular import issues."""
    exceptions_file = files("orbit").joinpath("exceptions.py")
    tree = ast.parse(exceptions_file.read_text())
    
    imports = [
        node.module for node in ast.walk(tree)
        if isinstance(node, ast.ImportFrom) and node.module
    ] + [
        alias.name for node in ast.walk(tree)
        if isinstance(node, ast.Import)
        for alias in node.names
    ]
    
    forbidden_imports = {"orbit.logging", "logging"}
    violations = forbidden_imports.intersection(imports)
    assert not violations, f"orbit.exceptions should not import {violations}"
