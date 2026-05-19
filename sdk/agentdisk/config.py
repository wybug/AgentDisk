"""AgentDisk SDK configuration."""

from dataclasses import dataclass, field


@dataclass
class ClientConfig:
    """SDK client configuration (gateway mode only).

    The SDK receives a pre-issued JWT from the gateway and uses it
    to authenticate all requests to the AgentDisk backend.
    """

    base_url: str
    token: str
    timeout: float = 30.0
