from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error_response import ErrorResponse
from ...models.subject_usage_event import SubjectUsageEvent
from ...types import UNSET, Response, Unset


def _get_kwargs(
    subject: str,
    *,
    limit: int | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["limit"] = limit

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/v1/subjects/{subject}/usageevents".format(
            subject=quote(str(subject), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ErrorResponse | list[SubjectUsageEvent] | None:
    if response.status_code == 200:
        response_200 = []
        _response_200 = response.json()
        for response_200_item_data in _response_200:
            response_200_item = SubjectUsageEvent.from_dict(response_200_item_data)

            response_200.append(response_200_item)

        return response_200

    if response.status_code == 400:
        response_400 = ErrorResponse.from_dict(response.json())

        return response_400

    if response.status_code == 500:
        response_500 = ErrorResponse.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ErrorResponse | list[SubjectUsageEvent]]:
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
    limit: int | Unset = UNSET,
) -> Response[ErrorResponse | list[SubjectUsageEvent]]:
    """List subject usage events

    Args:
        subject (str):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ErrorResponse | list[SubjectUsageEvent]]
    """

    kwargs = _get_kwargs(
        subject=subject,
        limit=limit,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    subject: str,
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = UNSET,
) -> ErrorResponse | list[SubjectUsageEvent] | None:
    """List subject usage events

    Args:
        subject (str):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ErrorResponse | list[SubjectUsageEvent]
    """

    return sync_detailed(
        subject=subject,
        client=client,
        limit=limit,
    ).parsed


async def asyncio_detailed(
    subject: str,
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = UNSET,
) -> Response[ErrorResponse | list[SubjectUsageEvent]]:
    """List subject usage events

    Args:
        subject (str):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ErrorResponse | list[SubjectUsageEvent]]
    """

    kwargs = _get_kwargs(
        subject=subject,
        limit=limit,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    subject: str,
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = UNSET,
) -> ErrorResponse | list[SubjectUsageEvent] | None:
    """List subject usage events

    Args:
        subject (str):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ErrorResponse | list[SubjectUsageEvent]
    """

    return (
        await asyncio_detailed(
            subject=subject,
            client=client,
            limit=limit,
        )
    ).parsed
