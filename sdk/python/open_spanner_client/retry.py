"""Opt-in retries for generated REST usage writes."""

from __future__ import annotations

import asyncio
import random
import time
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from datetime import UTC, datetime
from email.utils import parsedate_to_datetime
from typing import TypeVar

import httpx

from open_spanner_client.types import Response


@dataclass(frozen=True)
class RestRetryEvent:
    attempt: int
    delay_seconds: float
    error: httpx.TransportError | None = None
    response: Response[object] | None = None


@dataclass(frozen=True)
class RestRetryPolicy:
    max_attempts: int = 3
    initial_backoff_seconds: float = 0.1
    max_backoff_seconds: float = 5.0
    jitter: float = 0.2
    on_retry: Callable[[RestRetryEvent], None] | None = None


T = TypeVar("T")


def retry_usage_write(call: Callable[[], Response[T]], policy: RestRetryPolicy | None = None) -> Response[T]:
    """Retry a generated ``sync_detailed`` usage call."""
    normalized = _normalize(policy or RestRetryPolicy())
    for attempt in range(1, normalized.max_attempts + 1):
        response: Response[T] | None = None
        error: httpx.TransportError | None = None
        try:
            response = call()
            if not _retryable_status(response.status_code):
                return response
        except httpx.TransportError as caught:
            error = caught
        if attempt >= normalized.max_attempts:
            if error is not None:
                raise error
            return response  # type: ignore[return-value]
        delay = _delay(response, normalized, attempt)
        if normalized.on_retry is not None:
            normalized.on_retry(RestRetryEvent(attempt + 1, delay, error, response))  # type: ignore[arg-type]
        time.sleep(delay)
    raise RuntimeError("retry loop ended without a response")


async def retry_usage_write_async(
    call: Callable[[], Awaitable[Response[T]]], policy: RestRetryPolicy | None = None
) -> Response[T]:
    """Retry a generated ``asyncio_detailed`` usage call."""
    normalized = _normalize(policy or RestRetryPolicy())
    for attempt in range(1, normalized.max_attempts + 1):
        response: Response[T] | None = None
        error: httpx.TransportError | None = None
        try:
            response = await call()
            if not _retryable_status(response.status_code):
                return response
        except httpx.TransportError as caught:
            error = caught
        if attempt >= normalized.max_attempts:
            if error is not None:
                raise error
            return response  # type: ignore[return-value]
        delay = _delay(response, normalized, attempt)
        if normalized.on_retry is not None:
            normalized.on_retry(RestRetryEvent(attempt + 1, delay, error, response))  # type: ignore[arg-type]
        await asyncio.sleep(delay)
    raise RuntimeError("retry loop ended without a response")


def _normalize(policy: RestRetryPolicy) -> RestRetryPolicy:
    initial = max(0.001, policy.initial_backoff_seconds)
    return RestRetryPolicy(
        max_attempts=max(1, policy.max_attempts),
        initial_backoff_seconds=initial,
        max_backoff_seconds=max(initial, policy.max_backoff_seconds),
        jitter=min(1.0, max(0.0, policy.jitter)),
        on_retry=policy.on_retry,
    )


def _retryable_status(status: int) -> bool:
    return status in {429, 500, 502, 503, 504}


def _delay(response: Response[object] | None, policy: RestRetryPolicy, attempt: int) -> float:
    if response is not None:
        retry_after = response.headers.get("Retry-After", "").strip()
        try:
            seconds = float(retry_after)
            if seconds > 0:
                return seconds
        except ValueError:
            try:
                at = parsedate_to_datetime(retry_after)
                now = datetime.now(UTC)
                if at.tzinfo is None:
                    at = at.replace(tzinfo=UTC)
                if at > now:
                    return (at - now).total_seconds()
            except (TypeError, ValueError):
                pass
    backoff = min(policy.max_backoff_seconds, policy.initial_backoff_seconds * (2 ** (attempt - 1)))
    return max(0.001, backoff * (1 - policy.jitter + random.random() * 2 * policy.jitter))
