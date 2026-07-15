from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.consumption_decision import ConsumptionDecision
from ...models.error_response import ErrorResponse
from ...types import Response


def _get_kwargs(
    idempotency_key: str,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/v1/entitlements/decisions/{idempotency_key}".format(
            idempotency_key=quote(str(idempotency_key), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ConsumptionDecision | ErrorResponse | None:
    if response.status_code == 200:
        response_200 = ConsumptionDecision.from_dict(response.json())

        return response_200

    if response.status_code == 404:
        response_404 = ErrorResponse.from_dict(response.json())

        return response_404

    if response.status_code == 500:
        response_500 = ErrorResponse.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ConsumptionDecision | ErrorResponse]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    idempotency_key: str,
    *,
    client: AuthenticatedClient | Client,
) -> Response[ConsumptionDecision | ErrorResponse]:
    """Get consumption decision

    Args:
        idempotency_key (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ConsumptionDecision | ErrorResponse]
    """

    kwargs = _get_kwargs(
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    idempotency_key: str,
    *,
    client: AuthenticatedClient | Client,
) -> ConsumptionDecision | ErrorResponse | None:
    """Get consumption decision

    Args:
        idempotency_key (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ConsumptionDecision | ErrorResponse
    """

    return sync_detailed(
        idempotency_key=idempotency_key,
        client=client,
    ).parsed


async def asyncio_detailed(
    idempotency_key: str,
    *,
    client: AuthenticatedClient | Client,
) -> Response[ConsumptionDecision | ErrorResponse]:
    """Get consumption decision

    Args:
        idempotency_key (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ConsumptionDecision | ErrorResponse]
    """

    kwargs = _get_kwargs(
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    idempotency_key: str,
    *,
    client: AuthenticatedClient | Client,
) -> ConsumptionDecision | ErrorResponse | None:
    """Get consumption decision

    Args:
        idempotency_key (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ConsumptionDecision | ErrorResponse
    """

    return (
        await asyncio_detailed(
            idempotency_key=idempotency_key,
            client=client,
        )
    ).parsed
