from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.entitlement_progress import EntitlementProgress
from ...models.error_response import ErrorResponse
from ...types import Response


def _get_kwargs(
    subject: str,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/v1/plans/subjects/{subject}/progress".format(
            subject=quote(str(subject), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> EntitlementProgress | ErrorResponse | None:
    if response.status_code == 200:
        response_200 = EntitlementProgress.from_dict(response.json())

        return response_200

    if response.status_code == 400:
        response_400 = ErrorResponse.from_dict(response.json())

        return response_400

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
) -> Response[EntitlementProgress | ErrorResponse]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    subject: str,
    *,
    client: AuthenticatedClient | Client,
) -> Response[EntitlementProgress | ErrorResponse]:
    """Get subject plan progress

    Args:
        subject (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[EntitlementProgress | ErrorResponse]
    """

    kwargs = _get_kwargs(
        subject=subject,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    subject: str,
    *,
    client: AuthenticatedClient | Client,
) -> EntitlementProgress | ErrorResponse | None:
    """Get subject plan progress

    Args:
        subject (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        EntitlementProgress | ErrorResponse
    """

    return sync_detailed(
        subject=subject,
        client=client,
    ).parsed


async def asyncio_detailed(
    subject: str,
    *,
    client: AuthenticatedClient | Client,
) -> Response[EntitlementProgress | ErrorResponse]:
    """Get subject plan progress

    Args:
        subject (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[EntitlementProgress | ErrorResponse]
    """

    kwargs = _get_kwargs(
        subject=subject,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    subject: str,
    *,
    client: AuthenticatedClient | Client,
) -> EntitlementProgress | ErrorResponse | None:
    """Get subject plan progress

    Args:
        subject (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        EntitlementProgress | ErrorResponse
    """

    return (
        await asyncio_detailed(
            subject=subject,
            client=client,
        )
    ).parsed
