"""AgentDisk SDK exceptions."""


class AgentDiskError(Exception):
    """Base exception for AgentDisk SDK."""

    def __init__(self, code: int, message: str, http_status: int = 0):
        self.code = code
        self.message = message
        self.http_status = http_status
        super().__init__(f"[{code}] {message}")


class AuthError(AgentDiskError):
    """401 Unauthorized."""


class PermissionDeniedError(AgentDiskError):
    """403 Forbidden."""


class NotFoundError(AgentDiskError):
    """404 Not Found."""


class BadRequestError(AgentDiskError):
    """400 Bad Request."""


class ServerError(AgentDiskError):
    """500 Internal Server Error."""


def raise_for_response(http_status: int, body: dict) -> None:
    code = body.get("code", 0)
    message = body.get("message", "")
    if code == 0:
        return
    cls = {
        400: BadRequestError,
        401: AuthError,
        403: PermissionDeniedError,
        404: NotFoundError,
        500: ServerError,
    }.get(http_status, AgentDiskError)
    raise cls(code=code, message=message, http_status=http_status)
