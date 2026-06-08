from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="UsagePruneMeter")


@_attrs_define
class UsagePruneMeter:
    """
    Attributes:
        before (str | Unset):
        deleted (int | Unset):
        meter (str | Unset):
    """

    before: str | Unset = UNSET
    deleted: int | Unset = UNSET
    meter: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        before = self.before

        deleted = self.deleted

        meter = self.meter

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if before is not UNSET:
            field_dict["before"] = before
        if deleted is not UNSET:
            field_dict["deleted"] = deleted
        if meter is not UNSET:
            field_dict["meter"] = meter

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        before = d.pop("before", UNSET)

        deleted = d.pop("deleted", UNSET)

        meter = d.pop("meter", UNSET)

        usage_prune_meter = cls(
            before=before,
            deleted=deleted,
            meter=meter,
        )

        usage_prune_meter.additional_properties = d
        return usage_prune_meter

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
