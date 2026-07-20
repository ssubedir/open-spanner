from __future__ import annotations

import unittest
from http import HTTPStatus

import grpc

from open_spanner_client.grpc.pb.open_spanner.v1 import usage_pb2
from open_spanner_client.retry import RestRetryPolicy, retry_usage_write
from open_spanner_client.stream import Event, RetryPolicy, StreamClient, _retry_info_delay_seconds
from open_spanner_client.types import Response


class RetryableError(grpc.RpcError):
    def __init__(self, trailers=()) -> None:  # noqa: ANN001
        self._trailers = trailers

    def code(self) -> grpc.StatusCode:
        return grpc.StatusCode.RESOURCE_EXHAUSTED

    def trailing_metadata(self):  # noqa: ANN201
        return self._trailers


class Stub:
    def __init__(self) -> None:
        self.requests = []

    def CreateUsage(self, request, *, metadata):  # noqa: N802, ANN001, ANN201
        self.requests.append(request)
        if len(self.requests) == 1:
            raise RetryableError()
        return usage_pb2.CreateUsageResponse(
            event=usage_pb2.UsageEvent(
                id="evt_retry",
                idempotency_key=request.event.idempotency_key,
                subject=request.event.subject,
                meter=request.event.meter,
                quantity=request.event.quantity,
                timestamp=request.event.timestamp,
                metadata=request.event.metadata,
            )
        )


class StreamRetryTest(unittest.TestCase):
    def test_rest_retry_honors_retry_after(self) -> None:
        calls = 0
        retries = []

        def call() -> Response[object]:
            nonlocal calls
            calls += 1
            status = HTTPStatus.TOO_MANY_REQUESTS if calls == 1 else HTTPStatus.CREATED
            headers = {"Retry-After": "0.001"} if calls == 1 else {}
            return Response(status_code=status, content=b"", headers=headers, parsed=None)

        response = retry_usage_write(
            call,
            RestRetryPolicy(max_attempts=2, initial_backoff_seconds=0.05, jitter=0, on_retry=retries.append),
        )
        self.assertEqual(response.status_code, HTTPStatus.CREATED)
        self.assertEqual(calls, 2)
        self.assertEqual(retries[0].delay_seconds, 0.001)

    def test_retries_transient_unary_call_with_original_request(self) -> None:
        retries = []
        client = StreamClient(
            "localhost:1",
            "osp_test",
            retry_policy=RetryPolicy(
                max_attempts=2,
                initial_backoff_seconds=0.001,
                max_backoff_seconds=0.001,
                jitter=0,
                on_retry=retries.append,
            ),
        )
        stub = Stub()
        client._stub = stub
        try:
            result = client.track(Event(idempotency_key="idem_retry", subject="org", meter="requests", quantity=1))
        finally:
            client.close()

        self.assertEqual(result.id, "evt_retry")
        self.assertEqual(len(stub.requests), 2)
        self.assertIs(stub.requests[0], stub.requests[1])
        self.assertEqual(len(retries), 1)
        self.assertEqual(retries[0].attempt, 2)

    def test_reads_standard_retry_info_delay(self) -> None:
        duration = _field_varint(2, 1_000_000)
        retry_info = _field_bytes(1, duration)
        detail = _field_bytes(1, b"type.googleapis.com/google.rpc.RetryInfo") + _field_bytes(2, retry_info)
        encoded_status = _field_bytes(3, detail)
        error = RetryableError((("grpc-status-details-bin", encoded_status),))
        self.assertEqual(_retry_info_delay_seconds(error), 0.001)


def _field_bytes(number: int, value: bytes) -> bytes:
    return _varint(number << 3 | 2) + _varint(len(value)) + value


def _field_varint(number: int, value: int) -> bytes:
    return _varint(number << 3) + _varint(value)


def _varint(value: int) -> bytes:
    result = bytearray()
    while value >= 0x80:
        result.append((value & 0x7F) | 0x80)
        value >>= 7
    result.append(value)
    return bytes(result)


if __name__ == "__main__":
    unittest.main()
