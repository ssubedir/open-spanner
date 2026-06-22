from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="EntitlementCheckRequest")


@_attrs_define
class EntitlementCheckRequest:
    """
    Attributes:
        meter (str | Unset):
        quantity (float | Unset):
        subject (str | Unset):
    """

    meter: str | Unset = UNSET
    quantity: float | Unset = UNSET
    subject: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        meter = self.meter

        quantity = self.quantity

        subject = self.subject

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if meter is not UNSET:
            field_dict["meter"] = meter
        if quantity is not UNSET:
            field_dict["quantity"] = quantity
        if subject is not UNSET:
            field_dict["subject"] = subject

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        meter = d.pop("meter", UNSET)

        quantity = d.pop("quantity", UNSET)

        subject = d.pop("subject", UNSET)

        entitlement_check_request = cls(
            meter=meter,
            quantity=quantity,
            subject=subject,
        )

        entitlement_check_request.additional_properties = d
        return entitlement_check_request

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
