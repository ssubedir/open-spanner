from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error_response import ErrorResponse
from ...models.usage_dimension_value_list_response import UsageDimensionValueListResponse
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    meter: str,
    field: str,
    subject: str | Unset = UNSET,
    from_: str | Unset = UNSET,
    to: str | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["meter"] = meter

    params["field"] = field

    params["subject"] = subject

    params["from"] = from_

    params["to"] = to

    params["limit"] = limit

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/v1/usages/dimensions",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ErrorResponse | UsageDimensionValueListResponse | None:
    if response.status_code == 200:
        response_200 = UsageDimensionValueListResponse.from_dict(response.json())

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
) -> Response[ErrorResponse | UsageDimensionValueListResponse]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    meter: str,
    field: str,
    subject: str | Unset = UNSET,
    from_: str | Unset = UNSET,
    to: str | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> Response[ErrorResponse | UsageDimensionValueListResponse]:
    """List usage dimension values

    Args:
        meter (str):
        field (str):
        subject (str | Unset):
        from_ (str | Unset):
        to (str | Unset):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ErrorResponse | UsageDimensionValueListResponse]
    """

    kwargs = _get_kwargs(
        meter=meter,
        field=field,
        subject=subject,
        from_=from_,
        to=to,
        limit=limit,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    meter: str,
    field: str,
    subject: str | Unset = UNSET,
    from_: str | Unset = UNSET,
    to: str | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> ErrorResponse | UsageDimensionValueListResponse | None:
    """List usage dimension values

    Args:
        meter (str):
        field (str):
        subject (str | Unset):
        from_ (str | Unset):
        to (str | Unset):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ErrorResponse | UsageDimensionValueListResponse
    """

    return sync_detailed(
        client=client,
        meter=meter,
        field=field,
        subject=subject,
        from_=from_,
        to=to,
        limit=limit,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    meter: str,
    field: str,
    subject: str | Unset = UNSET,
    from_: str | Unset = UNSET,
    to: str | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> Response[ErrorResponse | UsageDimensionValueListResponse]:
    """List usage dimension values

    Args:
        meter (str):
        field (str):
        subject (str | Unset):
        from_ (str | Unset):
        to (str | Unset):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ErrorResponse | UsageDimensionValueListResponse]
    """

    kwargs = _get_kwargs(
        meter=meter,
        field=field,
        subject=subject,
        from_=from_,
        to=to,
        limit=limit,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    meter: str,
    field: str,
    subject: str | Unset = UNSET,
    from_: str | Unset = UNSET,
    to: str | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> ErrorResponse | UsageDimensionValueListResponse | None:
    """List usage dimension values

    Args:
        meter (str):
        field (str):
        subject (str | Unset):
        from_ (str | Unset):
        to (str | Unset):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ErrorResponse | UsageDimensionValueListResponse
    """

    return (
        await asyncio_detailed(
            client=client,
            meter=meter,
            field=field,
            subject=subject,
            from_=from_,
            to=to,
            limit=limit,
        )
    ).parsed
