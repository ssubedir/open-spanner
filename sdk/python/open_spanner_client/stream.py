"""gRPC streaming client for Open Spanner usage ingestion."""

from __future__ import annotations

import json
import random
import time
from collections.abc import Callable, Iterable, Mapping, Sequence
from dataclasses import dataclass
from datetime import UTC, datetime
from queue import Queue
from threading import Thread
from typing import Any, TypeVar

import grpc
from google.protobuf import json_format, struct_pb2, timestamp_pb2

from open_spanner_client.grpc.pb.open_spanner.v1 import usage_pb2, usage_pb2_grpc


@dataclass(frozen=True)
class Event:
    subject: str
    meter: str
    quantity: float
    idempotency_key: str = ""
    timestamp: datetime | None = None
    metadata: Mapping[str, Any] | None = None


@dataclass(frozen=True)
class RecordedEvent:
    id: str
    idempotency_key: str
    subject: str
    meter: str
    quantity: float
    timestamp: datetime | None
    received_at: datetime | None
    metadata: dict[str, Any]


@dataclass(frozen=True)
class Failure:
    index: int
    code: str
    message: str


@dataclass(frozen=True)
class BulkResult:
    accepted_count: int
    duplicate_count: int
    failed_count: int
    accepted: list[RecordedEvent]
    duplicates: list[RecordedEvent]
    failed: list[Failure]


@dataclass(frozen=True)
class RetryEvent:
    attempt: int
    delay_seconds: float
    error: grpc.RpcError


@dataclass(frozen=True)
class RetryPolicy:
    """Opt-in unary ingestion retry policy. max_attempts includes the first call."""

    max_attempts: int = 3
    initial_backoff_seconds: float = 0.1
    max_backoff_seconds: float = 5.0
    jitter: float = 0.2
    on_retry: Callable[[RetryEvent], None] | None = None


class StreamClient:
    def __init__(
        self,
        address: str,
        api_key: str,
        *,
        credentials: grpc.ChannelCredentials | None = None,
        options: Sequence[tuple[str, Any]] | None = None,
        retry_policy: RetryPolicy | None = None,
    ) -> None:
        address = address.strip()
        if not address:
            raise ValueError("gRPC address is required")

        api_key = api_key.strip()
        if not api_key:
            raise ValueError("API key is required")

        self._api_key = api_key
        self._retry_policy = _normalize_retry_policy(retry_policy) if retry_policy is not None else None
        if credentials is None:
            self._channel = grpc.insecure_channel(address, options=options)
        else:
            self._channel = grpc.secure_channel(address, credentials, options=options)
        self._stub = usage_pb2_grpc.UsageServiceStub(self._channel)

    def close(self) -> None:
        self._channel.close()

    def track(self, event: Event) -> RecordedEvent:
        request = usage_pb2.CreateUsageRequest(event=_event_input(event))
        response = _retry_unary(
            lambda: self._stub.CreateUsage(request, metadata=self._metadata()),
            self._retry_policy,
        )
        return _recorded_event(response.event)

    def track_bulk(self, idempotency_key: str, events: Iterable[Event]) -> BulkResult:
        request = usage_pb2.CreateUsageBulkRequest(
            idempotency_key=idempotency_key,
            events=[_event_input(event) for event in events],
        )
        response = _retry_unary(
            lambda: self._stub.CreateUsageBulk(request, metadata=self._metadata()),
            self._retry_policy,
        )
        return _bulk_result(response)

    def stream(self, idempotency_key: str) -> UsageStream:
        return UsageStream(self._stub, self._metadata((("idempotency-key", idempotency_key),)))

    def _metadata(self, extra: Sequence[tuple[str, str]] = ()) -> tuple[tuple[str, str], ...]:
        return (("authorization", f"Bearer {self._api_key}"), *extra)


class UsageStream:
    def __init__(self, stub: usage_pb2_grpc.UsageServiceStub, metadata: tuple[tuple[str, str], ...]) -> None:
        self._queue: Queue[usage_pb2.StreamUsageRequest | None] = Queue()
        self._closed = False
        self._result: BulkResult | None = None
        self._error: BaseException | None = None
        self._thread = Thread(target=self._run, args=(stub, metadata), daemon=True)
        self._thread.start()

    def track(self, event: Event) -> None:
        if self._closed:
            raise RuntimeError("stream is already closed")
        self._queue.put(usage_pb2.StreamUsageRequest(event=_event_input(event)))

    def close(self) -> BulkResult:
        if not self._closed:
            self._closed = True
            self._queue.put(None)

        self._thread.join()
        if self._error is not None:
            raise self._error
        if self._result is None:
            raise RuntimeError("stream closed without a response")
        return self._result

    def _run(self, stub: usage_pb2_grpc.UsageServiceStub, metadata: tuple[tuple[str, str], ...]) -> None:
        try:
            response = stub.StreamUsage(self._requests(), metadata=metadata)
            self._result = _bulk_result(response)
        except BaseException as error:
            self._error = error

    def _requests(self) -> Iterable[usage_pb2.StreamUsageRequest]:
        while True:
            item = self._queue.get()
            if item is None:
                return
            yield item


def _event_input(event: Event) -> usage_pb2.UsageEventInput:
    timestamp = timestamp_pb2.Timestamp()
    timestamp.FromDatetime(_event_time(event.timestamp))
    return usage_pb2.UsageEventInput(
        idempotency_key=event.idempotency_key,
        subject=event.subject,
        meter=event.meter,
        quantity=event.quantity,
        timestamp=timestamp,
        metadata=_metadata_values(event.metadata or {}),
    )


def _event_time(value: datetime | None) -> datetime:
    if value is None:
        return datetime.now(UTC)
    if value.tzinfo is None:
        return value.replace(tzinfo=UTC)
    return value.astimezone(UTC)


def _metadata_values(fields: Mapping[str, Any]) -> dict[str, struct_pb2.Value]:
    values: dict[str, struct_pb2.Value] = {}
    for key, value in fields.items():
        proto_value = struct_pb2.Value()
        json_format.Parse(json.dumps(value), proto_value)
        values[key] = proto_value
    return values


def _bulk_result(response: usage_pb2.CreateUsageBulkResponse | usage_pb2.StreamUsageResponse) -> BulkResult:
    return BulkResult(
        accepted_count=response.accepted_count,
        duplicate_count=response.duplicate_count,
        failed_count=response.failed_count,
        accepted=[_recorded_event(event) for event in response.accepted],
        duplicates=[_recorded_event(event) for event in response.duplicates],
        failed=[Failure(index=item.index, code=item.code, message=item.message) for item in response.failed],
    )


def _recorded_event(event: usage_pb2.UsageEvent) -> RecordedEvent:
    return RecordedEvent(
        id=event.id,
        idempotency_key=event.idempotency_key,
        subject=event.subject,
        meter=event.meter,
        quantity=event.quantity,
        timestamp=_timestamp_datetime(event.timestamp),
        received_at=_timestamp_datetime(event.received_at),
        metadata={key: json_format.MessageToDict(value) for key, value in event.metadata.items()},
    )


def _timestamp_datetime(value: timestamp_pb2.Timestamp) -> datetime | None:
    if value.seconds == 0 and value.nanos == 0:
        return None
    return value.ToDatetime().replace(tzinfo=UTC)


T = TypeVar("T")


def _normalize_retry_policy(policy: RetryPolicy) -> RetryPolicy:
    initial = max(0.001, policy.initial_backoff_seconds)
    return RetryPolicy(
        max_attempts=max(1, policy.max_attempts),
        initial_backoff_seconds=initial,
        max_backoff_seconds=max(initial, policy.max_backoff_seconds),
        jitter=min(1.0, max(0.0, policy.jitter)),
        on_retry=policy.on_retry,
    )


def _retry_unary(call: Callable[[], T], policy: RetryPolicy | None) -> T:
    if policy is None:
        return call()
    for attempt in range(1, policy.max_attempts + 1):
        try:
            return call()
        except grpc.RpcError as error:
            if attempt >= policy.max_attempts or not _retryable_error(error):
                raise
            delay = _retry_delay_seconds(error, policy, attempt)
            if policy.on_retry is not None:
                policy.on_retry(RetryEvent(attempt=attempt + 1, delay_seconds=delay, error=error))
            time.sleep(delay)
    raise RuntimeError("retry loop ended without a result")


def _retryable_error(error: grpc.RpcError) -> bool:
    return error.code() in {
        grpc.StatusCode.RESOURCE_EXHAUSTED,
        grpc.StatusCode.UNAVAILABLE,
        grpc.StatusCode.DEADLINE_EXCEEDED,
    }


def _retry_delay_seconds(error: grpc.RpcError, policy: RetryPolicy, attempt: int) -> float:
    guided = _retry_info_delay_seconds(error)
    if guided is not None and guided > 0:
        return guided
    backoff = min(policy.max_backoff_seconds, policy.initial_backoff_seconds * (2 ** (attempt - 1)))
    factor = 1 - policy.jitter + random.random() * 2 * policy.jitter
    return max(0.001, backoff * factor)


def _retry_info_delay_seconds(error: grpc.RpcError) -> float | None:
    for key, value in error.trailing_metadata() or ():
        if key != "grpc-status-details-bin" or not isinstance(value, bytes):
            continue
        try:
            for detail in _protobuf_bytes(value, 3):
                type_url = next(iter(_protobuf_bytes(detail, 1)), b"").decode(errors="replace")
                payload = next(iter(_protobuf_bytes(detail, 2)), b"")
                if not type_url.endswith("google.rpc.RetryInfo") or not payload:
                    continue
                duration = next(iter(_protobuf_bytes(payload, 1)), b"")
                if not duration:
                    continue
                seconds = _protobuf_varint(duration, 1) or 0
                nanos = _protobuf_varint(duration, 2) or 0
                return seconds + nanos / 1_000_000_000
        except ValueError:
            continue
    return None


def _protobuf_bytes(data: bytes, wanted_field: int) -> list[bytes]:
    results: list[bytes] = []
    offset = 0
    while offset < len(data):
        tag, offset = _read_varint(data, offset)
        field, wire = tag >> 3, tag & 7
        if wire == 2:
            length, offset = _read_varint(data, offset)
            end = offset + length
            if field == wanted_field:
                results.append(data[offset:end])
            offset = end
        elif wire == 0:
            _, offset = _read_varint(data, offset)
        else:
            break
    return results


def _protobuf_varint(data: bytes, wanted_field: int) -> int | None:
    offset = 0
    while offset < len(data):
        tag, offset = _read_varint(data, offset)
        field, wire = tag >> 3, tag & 7
        if wire == 0:
            value, offset = _read_varint(data, offset)
            if field == wanted_field:
                return value
        elif wire == 2:
            length, offset = _read_varint(data, offset)
            offset += length
        else:
            return None
    return None


def _read_varint(data: bytes, offset: int) -> tuple[int, int]:
    value = 0
    shift = 0
    while offset < len(data) and shift < 70:
        byte = data[offset]
        offset += 1
        value |= (byte & 0x7F) << shift
        if byte & 0x80 == 0:
            return value, offset
        shift += 7
    raise ValueError("invalid protobuf varint")
