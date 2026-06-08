from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.usage_bucket_group import UsageBucketGroup


T = TypeVar("T", bound="UsageBucket")


@_attrs_define
class UsageBucket:
    """
    Attributes:
        aggregation (str | Unset):
        bucket_size (str | Unset):
        bucket_start (str | Unset):
        group (UsageBucketGroup | Unset):
        meter (str | Unset):
        quantity (float | Unset):
        subject (str | Unset):
        unit (str | Unset):
    """

    aggregation: str | Unset = UNSET
    bucket_size: str | Unset = UNSET
    bucket_start: str | Unset = UNSET
    group: UsageBucketGroup | Unset = UNSET
    meter: str | Unset = UNSET
    quantity: float | Unset = UNSET
    subject: str | Unset = UNSET
    unit: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        aggregation = self.aggregation

        bucket_size = self.bucket_size

        bucket_start = self.bucket_start

        group: dict[str, Any] | Unset = UNSET
        if not isinstance(self.group, Unset):
            group = self.group.to_dict()

        meter = self.meter

        quantity = self.quantity

        subject = self.subject

        unit = self.unit

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if aggregation is not UNSET:
            field_dict["aggregation"] = aggregation
        if bucket_size is not UNSET:
            field_dict["bucket_size"] = bucket_size
        if bucket_start is not UNSET:
            field_dict["bucket_start"] = bucket_start
        if group is not UNSET:
            field_dict["group"] = group
        if meter is not UNSET:
            field_dict["meter"] = meter
        if quantity is not UNSET:
            field_dict["quantity"] = quantity
        if subject is not UNSET:
            field_dict["subject"] = subject
        if unit is not UNSET:
            field_dict["unit"] = unit

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.usage_bucket_group import UsageBucketGroup

        d = dict(src_dict)
        aggregation = d.pop("aggregation", UNSET)

        bucket_size = d.pop("bucket_size", UNSET)

        bucket_start = d.pop("bucket_start", UNSET)

        _group = d.pop("group", UNSET)
        group: UsageBucketGroup | Unset
        if isinstance(_group, Unset):
            group = UNSET
        else:
            group = UsageBucketGroup.from_dict(_group)

        meter = d.pop("meter", UNSET)

        quantity = d.pop("quantity", UNSET)

        subject = d.pop("subject", UNSET)

        unit = d.pop("unit", UNSET)

        usage_bucket = cls(
            aggregation=aggregation,
            bucket_size=bucket_size,
            bucket_start=bucket_start,
            group=group,
            meter=meter,
            quantity=quantity,
            subject=subject,
            unit=unit,
        )

        usage_bucket.additional_properties = d
        return usage_bucket

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
