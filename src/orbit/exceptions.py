"""Custom exceptions."""

class OrbitError(Exception):
    """Base class for all custom exceptions in the Orbit application."""
    pass


class InstallationError(OrbitError):
    """Application is not correctly installed or initialized."""
    pass

class NotInitializedError(InstallationError):
    """Orbit has not been initialized """
    pass

class AlreadyInitializedError(InstallationError):
    """Orbit is already initialized (use --force to reinitialize)."""
    pass


class ConfigurationError(OrbitError):
    """Configuration file issues."""
    pass

# Database/Storage
class DatabaseError(OrbitError):
    """Database access or integrity issues."""
    pass

class SchemaError(DatabaseError):
    """Database schema is missing or corrupted."""
    pass


#region CLI errors
class ValidationError(OrbitError):
    """Invalid input or parameters."""
    pass

class ConflictingOptionsError(ValidationError):
    """Mutually exclusive options were provided."""
    pass

#endregion
