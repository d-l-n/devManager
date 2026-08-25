import os

def normalize_path(path: str) -> str:
    """Replace backslashes with forward slashes and strip trailing slashes."""
    if not path:
        return path
    normalized = path.replace('\\', '/')
    while normalized.endswith('/') and len(normalized) > 1:
        normalized = normalized[:-1]
    return normalized

def is_valid_project_path(path: str) -> bool:
    """Check if the path exists and is a directory."""
    if not path:
        return False
    # Use normalized path for cross-platform compatibility
    normalized_path = normalize_path(path)
    return os.path.isdir(normalized_path)
