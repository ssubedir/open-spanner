from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error_response import ErrorResponse
from ...models.usage_bulk_result import UsageBulkResult
from ...models.usage_create_request import UsageCreateRequest
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    body: list[UsageCreateRequest],
    idempotency_key: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/v1/usages/bulk",
    }

    _kwargs["json"] = []
    for body_item_data in body:
        body_item = body_item_data.to_dict()
        _kwargs["json"].append(body_item)

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ErrorResponse | UsageBulkResult | None:
    if response.status_code == 201:
        response_201 = UsageBulkResult.from_dict(response.json())

        return response_201

    if response.status_code == 400:
        response_400 = ErrorResponse.from_dict(response.json())

        return response_400

    if response.status_code == 404:
        response_404 = ErrorResponse.from_dict(response.json())

        return response_404

    if response.status_code == 409:
        response_409 = ErrorResponse.from_dict(response.json())

        return response_409

    if response.status_code == 500:
        response_500 = ErrorResponse.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ErrorResponse | UsageBulkResult]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: list[UsageCreateRequest],
    idempotency_key: str | Unset = UNSET,
) -> Response[ErrorResponse | UsageBulkResult]:
    """Create usage in bulk

     Records up to 1000 usage events. The Idempotency-Key header replays the original bulk response for
    the same batch. Per-event idempotency_key values replay existing events as duplicates. Duplicate
    event IDs are conflicts.

    Args:
        idempotency_key (str | Unset):
        body (list[UsageCreateRequest]):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ErrorResponse | UsageBulkResult]
    """

    kwargs = _get_kwargs(
        body=body,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: list[UsageCreateRequest],
    idempotency_key: str | Unset = UNSET,
) -> ErrorResponse | UsageBulkResult | None:
    """Create usage in bulk

     Records up to 1000 usage events. The Idempotency-Key header replays the original bulk response for
    the same batch. Per-event idempotency_key values replay existing events as duplicates. Duplicate
    event IDs are conflicts.

    Args:
        idempotency_key (str | Unset):
        body (list[UsageCreateRequest]):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ErrorResponse | UsageBulkResult
    """

    return sync_detailed(
        client=client,
        body=body,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: list[UsageCreateRequest],
    idempotency_key: str | Unset = UNSET,
) -> Response[ErrorResponse | UsageBulkResult]:
    """Create usage in bulk

     Records up to 1000 usage events. The Idempotency-Key header replays the original bulk response for
    the same batch. Per-event idempotency_key values replay existing events as duplicates. Duplicate
    event IDs are conflicts.

    Args:
        idempotency_key (str | Unset):
        body (list[UsageCreateRequest]):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ErrorResponse | UsageBulkResult]
    """

    kwargs = _get_kwargs(
        body=body,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: list[UsageCreateRequest],
    idempotency_key: str | Unset = UNSET,
) -> ErrorResponse | UsageBulkResult | None:
    """Create usage in bulk

     Records up to 1000 usage events. The Idempotency-Key header replays the original bulk response for
    the same batch. Per-event idempotency_key values replay existing events as duplicates. Duplicate
    event IDs are conflicts.

    Args:
        idempotency_key (str | Unset):
        body (list[UsageCreateRequest]):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ErrorResponse | UsageBulkResult
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            idempotency_key=idempotency_key,
        )
    ).parsed
