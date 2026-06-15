from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.internal_metering_adapters_http_usage_filter_request import (
        InternalMeteringAdaptersHttpUsageFilterRequest,
    )


T = TypeVar("T", bound="InternalMeteringAdaptersHttpUsageSearchRequest")


@_attrs_define
class InternalMeteringAdaptersHttpUsageSearchRequest:
    """
    Attributes:
        bucket_size (str | Unset):
        filter_ (InternalMeteringAdaptersHttpUsageFilterRequest | Unset):
        from_ (str | Unset):
        group_by (list[str] | Unset):
        limit (int | Unset):
        meter (str | Unset):
        subject (str | Unset):
        to (str | Unset):
    """

    bucket_size: str | Unset = UNSET
    filter_: InternalMeteringAdaptersHttpUsageFilterRequest | Unset = UNSET
    from_: str | Unset = UNSET
    group_by: list[str] | Unset = UNSET
    limit: int | Unset = UNSET
    meter: str | Unset = UNSET
    subject: str | Unset = UNSET
    to: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        bucket_size = self.bucket_size

        filter_: dict[str, Any] | Unset = UNSET
        if not isinstance(self.filter_, Unset):
            filter_ = self.filter_.to_dict()

        from_ = self.from_

        group_by: list[str] | Unset = UNSET
        if not isinstance(self.group_by, Unset):
            group_by = self.group_by

        limit = self.limit

        meter = self.meter

        subject = self.subject

        to = self.to

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if bucket_size is not UNSET:
            field_dict["bucket_size"] = bucket_size
        if filter_ is not UNSET:
            field_dict["filter"] = filter_
        if from_ is not UNSET:
            field_dict["from"] = from_
        if group_by is not UNSET:
            field_dict["group_by"] = group_by
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
        bucket_size = d.pop("bucket_size", UNSET)

        _filter_ = d.pop("filter", UNSET)
        filter_: InternalMeteringAdaptersHttpUsageFilterRequest | Unset
        if isinstance(_filter_, Unset):
            filter_ = UNSET
        else:
            filter_ = InternalMeteringAdaptersHttpUsageFilterRequest.from_dict(_filter_)

        from_ = d.pop("from", UNSET)

        group_by = cast(list[str], d.pop("group_by", UNSET))

        limit = d.pop("limit", UNSET)

        meter = d.pop("meter", UNSET)

        subject = d.pop("subject", UNSET)

        to = d.pop("to", UNSET)

        internal_metering_adapters_http_usage_search_request = cls(
            bucket_size=bucket_size,
            filter_=filter_,
            from_=from_,
            group_by=group_by,
            limit=limit,
            meter=meter,
            subject=subject,
            to=to,
        )

        internal_metering_adapters_http_usage_search_request.additional_properties = d
        return internal_metering_adapters_http_usage_search_request

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
