from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="UsageBreakdown")


@_attrs_define
class UsageBreakdown:
    """
    Attributes:
        aggregation (str | Unset):
        events (int | Unset):
        field (str | Unset):
        quantity (float | Unset):
        unit (str | Unset):
        value (str | Unset):
    """

    aggregation: str | Unset = UNSET
    events: int | Unset = UNSET
    field: str | Unset = UNSET
    quantity: float | Unset = UNSET
    unit: str | Unset = UNSET
    value: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        aggregation = self.aggregation

        events = self.events

        field = self.field

        quantity = self.quantity

        unit = self.unit

        value = self.value

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if aggregation is not UNSET:
            field_dict["aggregation"] = aggregation
        if events is not UNSET:
            field_dict["events"] = events
        if field is not UNSET:
            field_dict["field"] = field
        if quantity is not UNSET:
            field_dict["quantity"] = quantity
        if unit is not UNSET:
            field_dict["unit"] = unit
        if value is not UNSET:
            field_dict["value"] = value

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        aggregation = d.pop("aggregation", UNSET)

        events = d.pop("events", UNSET)

        field = d.pop("field", UNSET)

        quantity = d.pop("quantity", UNSET)

        unit = d.pop("unit", UNSET)

        value = d.pop("value", UNSET)

        usage_breakdown = cls(
            aggregation=aggregation,
            events=events,
            field=field,
            quantity=quantity,
            unit=unit,
            value=value,
        )

        usage_breakdown.additional_properties = d
        return usage_breakdown

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
