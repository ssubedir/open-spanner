from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.internal_metering_adapters_http_usage_filter_request import (
        InternalMeteringAdaptersHttpUsageFilterRequest,
    )


T = TypeVar("T", bound="InternalMeteringAdaptersHttpUsageEventSearchRequest")


@_attrs_define
class InternalMeteringAdaptersHttpUsageEventSearchRequest:
    """
    Attributes:
        cursor (str | Unset):
        filter_ (InternalMeteringAdaptersHttpUsageFilterRequest | Unset):
        from_ (str | Unset):
        limit (int | Unset):
        meter (str | Unset):
        subject (str | Unset):
        to (str | Unset):
    """

    cursor: str | Unset = UNSET
    filter_: InternalMeteringAdaptersHttpUsageFilterRequest | Unset = UNSET
    from_: str | Unset = UNSET
    limit: int | Unset = UNSET
    meter: str | Unset = UNSET
    subject: str | Unset = UNSET
    to: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        cursor = self.cursor

        filter_: dict[str, Any] | Unset = UNSET
        if not isinstance(self.filter_, Unset):
            filter_ = self.filter_.to_dict()

        from_ = self.from_

        limit = self.limit

        meter = self.meter

        subject = self.subject

        to = self.to

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if cursor is not UNSET:
            field_dict["cursor"] = cursor
        if filter_ is not UNSET:
            field_dict["filter"] = filter_
        if from_ is not UNSET:
            field_dict["from"] = from_
        if limit is not UNSET:
            field_dict["limit"] = limit
        if meter is not UNSET:
            field_dict["meter"] = meter
        if subject is not UNSET:
            field_dict["subject"] = subject
        if to is not UNSET:
            field_dict["to"] = to

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.internal_metering_adapters_http_usage_filter_request import (
            InternalMeteringAdaptersHttpUsageFilterRequest,
        )

        d = dict(src_dict)
        cursor = d.pop("cursor", UNSET)

        _filter_ = d.pop("filter", UNSET)
        filter_: InternalMeteringAdaptersHttpUsageFilterRequest | Unset
        if isinstance(_filter_, Unset):
            filter_ = UNSET
        else:
            filter_ = InternalMeteringAdaptersHttpUsageFilterRequest.from_dict(_filter_)

        from_ = d.pop("from", UNSET)

        limit = d.pop("limit", UNSET)

        meter = d.pop("meter", UNSET)

        subject = d.pop("subject", UNSET)

        to = d.pop("to", UNSET)

        internal_metering_adapters_http_usage_event_search_request = cls(
            cursor=cursor,
            filter_=filter_,
            from_=from_,
            limit=limit,
            meter=meter,
            subject=subject,
            to=to,
        )

        internal_metering_adapters_http_usage_event_search_request.additional_properties = d
        return internal_metering_adapters_http_usage_event_search_request

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
