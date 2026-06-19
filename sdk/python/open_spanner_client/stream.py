"""gRPC streaming client for Open Spanner usage ingestion."""

from __future__ import annotations

import json
from collections.abc import Iterable, Mapping, Sequence
from dataclasses import dataclass
from datetime import UTC, datetime
from queue import Queue
from threading import Thread
from typing import Any

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


class StreamClient:
    def __init__(
        self,
        address: str,
        api_key: str,
        *,
        credentials: grpc.ChannelCredentials | None = None,
        options: Sequence[tuple[str, Any]] | None = None,
    ) -> None:
        address = address.strip()
        if not address:
            raise ValueError("gRPC address is required")

        api_key = api_key.strip()
        if not api_key:
            raise ValueError("API key is required")

        self._api_key = api_key
        if credentials is None:
            self._channel = grpc.insecure_channel(address, options=options)
        else:
            self._channel = grpc.secure_channel(address, credentials, options=options)
        self._stub = usage_pb2_grpc.UsageServiceStub(self._channel)

    def close(self) -> None:
        self._channel.close()

    def track(self, event: Event) -> RecordedEvent:
        response = self._stub.CreateUsage(
            usage_pb2.CreateUsageRequest(event=_event_input(event)),
            metadata=self._metadata(),
        )
        return _recorded_event(response.event)

    def track_bulk(self, idempotency_key: str, events: Iterable[Event]) -> BulkResult:
        response = self._stub.CreateUsageBulk(
            usage_pb2.CreateUsageBulkRequest(
                idempotency_key=idempotency_key,
                events=[_event_input(event) for event in events],
            ),
            metadata=self._metadata(),
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
