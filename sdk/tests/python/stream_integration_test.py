from __future__ import annotations

from datetime import UTC, datetime, timedelta
import os
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import time
import unittest

import httpx

from open_spanner_client.stream import Event, StreamClient


class PythonStreamClientIntegrationTest(unittest.TestCase):
    def test_stream_client_records_bulk_and_streamed_usage(self) -> None:
        http_addr = free_tcp_addr()
        grpc_addr = free_tcp_addr()

        with start_open_spanner(http_addr, grpc_addr) as service:
            suffix = str(time.time_ns())
            api_key = create_api_key(service.base_url, suffix)
            meter_name = f"sdk_python_stream_requests_{suffix}"
            create_meter(service.base_url, api_key, meter_name)

            client = StreamClient(grpc_addr, api_key)
            try:
                now = datetime.now(UTC)
                bulk = client.track_bulk(
                    f"sdk-python-stream-bulk-{suffix}",
                    [
                        usage_event(
                            f"sdk-python-stream-bulk-{suffix}-1",
                            f"org_sdk_python_stream_{suffix}",
                            meter_name,
                            2,
                            now,
                            {"endpoint": "/orders", "status": 200},
                        ),
                        usage_event(
                            f"sdk-python-stream-bulk-{suffix}-2",
                            f"org_sdk_python_stream_{suffix}",
                            meter_name,
                            3,
                            now + timedelta(seconds=1),
                            {"endpoint": "/users", "status": 201},
                        ),
                    ],
                )

                self.assertEqual(bulk.accepted_count, 2)
                self.assertEqual(bulk.duplicate_count, 0)
                self.assertEqual(bulk.failed_count, 0)

                usage_stream = client.stream(f"sdk-python-stream-{suffix}")
                usage_stream.track(
                    usage_event(
                        f"sdk-python-stream-{suffix}-1",
                        f"org_sdk_python_stream_{suffix}",
                        meter_name,
                        7,
                        now + timedelta(seconds=2),
                        {"endpoint": "/checkout", "status": 200},
                    )
                )
                streamed = usage_stream.close()

                self.assertEqual(streamed.accepted_count, 1)
                self.assertEqual(streamed.duplicate_count, 0)
                self.assertEqual(streamed.failed_count, 0)

                events = list_usage_events(service.base_url, api_key, meter_name)
                self.assertEqual(len(events["items"]), 3)
            finally:
                client.close()


class OpenSpannerService:
    def __init__(self, process: subprocess.Popen[bytes], base_url: str, temp_dir: tempfile.TemporaryDirectory[str]) -> None:
        self.process = process
        self.base_url = base_url
        self._temp_dir = temp_dir

    def __enter__(self) -> OpenSpannerService:
        return self

    def __exit__(self, exc_type: object, exc: object, traceback: object) -> None:
        self.stop()

    def stop(self) -> None:
        if self.process.poll() is None:
            kill_process_tree(self.process)
            try:
                self.process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.process.kill()
                self.process.wait(timeout=5)
        if self.process.stdout is not None:
            self.process.stdout.close()
        self._temp_dir.cleanup()


def start_open_spanner(http_addr: str, grpc_addr: str) -> OpenSpannerService:
    repo_root = Path(__file__).resolve().parents[3]
    temp_dir = tempfile.TemporaryDirectory(prefix="open-spanner-sdk-python-")
    temp_path = Path(temp_dir.name)
    binary_path = temp_path / ("open-spanner-sdk-test.exe" if sys.platform == "win32" else "open-spanner-sdk-test")

    env = os.environ.copy()
    env["GOCACHE"] = str(repo_root / ".tmp" / "go-build")
    subprocess.run(
        ["go", "build", "-o", str(binary_path), "./cmd/api"],
        cwd=repo_root,
        env=env,
        check=True,
        text=True,
        capture_output=True,
    )

    env.update(
        {
            "OPEN_SPANNER_HTTP_ADDR": http_addr,
            "OPEN_SPANNER_GRPC_ADDR": grpc_addr,
            "OPEN_SPANNER_DB_DRIVER": "sqlite",
            "OPEN_SPANNER_SQLITE_PATH": str(temp_path / "open-spanner.db"),
            "OPEN_SPANNER_EXPORT_STORAGE_PATH": str(temp_path / "exports"),
        }
    )
    process = subprocess.Popen(
        [str(binary_path)],
        cwd=repo_root,
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
    )

    service = OpenSpannerService(process, f"http://{http_addr}", temp_dir)
    try:
        wait_for_ready(service)
    except BaseException:
        service.stop()
        raise
    return service


def wait_for_ready(service: OpenSpannerService) -> None:
    deadline = time.monotonic() + 20
    while time.monotonic() < deadline:
        if service.process.poll() is not None:
            output = service.process.stdout.read().decode("utf-8", errors="replace") if service.process.stdout else ""
            raise AssertionError(f"API process exited before ready:\n{output}")
        try:
            response = httpx.get(f"{service.base_url}/ready", timeout=1)
            if response.status_code == 204:
                return
        except httpx.HTTPError:
            pass
        time.sleep(0.1)
    raise AssertionError("API did not become ready")


def create_api_key(base_url: str, suffix: str) -> str:
    email = f"sdk-python-stream+{suffix}@example.com"
    password = "strong-password"
    with httpx.Client(timeout=5) as client:
        response = client.post(f"{base_url}/v1/auth/users", json={"email": email, "password": password})
        response.raise_for_status()

        response = client.post(f"{base_url}/v1/auth/sessions", json={"email": email, "password": password})
        response.raise_for_status()

        response = client.post(f"{base_url}/v1/auth/api-keys", json={"name": f"sdk python stream test {suffix}"})
        response.raise_for_status()
        return response.json()["key"]


def create_meter(base_url: str, api_key: str, meter_name: str) -> None:
    response = httpx.post(
        f"{base_url}/v1/meters",
        headers={"Authorization": f"Bearer {api_key}"},
        json={
            "name": meter_name,
            "description": "Python SDK stream integration requests",
            "unit": "request",
            "aggregation": "sum",
            "event_retention_days": 30,
            "dimensions": [
                {"name": "endpoint", "type": "string", "required": True},
                {"name": "status", "type": "number", "required": True},
            ],
        },
        timeout=5,
    )
    response.raise_for_status()


def list_usage_events(base_url: str, api_key: str, meter_name: str) -> dict[str, object]:
    response = httpx.get(
        f"{base_url}/v1/usageevents",
        headers={"Authorization": f"Bearer {api_key}"},
        params={"meter": meter_name, "limit": 10},
        timeout=5,
    )
    response.raise_for_status()
    return response.json()


def usage_event(
    idempotency_key: str,
    subject: str,
    meter_name: str,
    quantity: float,
    timestamp: datetime,
    metadata: dict[str, object],
) -> Event:
    return Event(
        idempotency_key=idempotency_key,
        subject=subject,
        meter=meter_name,
        quantity=quantity,
        timestamp=timestamp,
        metadata=metadata,
    )


def free_tcp_addr() -> str:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as server:
        server.bind(("127.0.0.1", 0))
        host, port = server.getsockname()
        return f"{host}:{port}"


def kill_process_tree(process: subprocess.Popen[bytes]) -> None:
    if sys.platform == "win32":
        subprocess.run(["taskkill", "/PID", str(process.pid), "/T", "/F"], check=False, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        return
    process.kill()


if __name__ == "__main__":
    unittest.main()
