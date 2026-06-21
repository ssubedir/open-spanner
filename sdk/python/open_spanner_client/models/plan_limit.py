from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="PlanLimit")


@_attrs_define
class PlanLimit:
    """
    Attributes:
        created_at (str | Unset):
        id (str | Unset):
        limit (float | Unset):
        meter (str | Unset):
        period (str | Unset):
        updated_at (str | Unset):
        warning_percent (float | Unset):
    """

    created_at: str | Unset = UNSET
    id: str | Unset = UNSET
    limit: float | Unset = UNSET
    meter: str | Unset = UNSET
    period: str | Unset = UNSET
    updated_at: str | Unset = UNSET
    warning_percent: float | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        created_at = self.created_at

        id = self.id

        limit = self.limit

        meter = self.meter

        period = self.period

        updated_at = self.updated_at

        warning_percent = self.warning_percent

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if created_at is not UNSET:
            field_dict["created_at"] = created_at
        if id is not UNSET:
            field_dict["id"] = id
        if limit is not UNSET:
            field_dict["limit"] = limit
        if meter is not UNSET:
            field_dict["meter"] = meter
        if period is not UNSET:
            field_dict["period"] = period
        if updated_at is not UNSET:
            field_dict["updated_at"] = updated_at
        if warning_percent is not UNSET:
            field_dict["warning_percent"] = warning_percent

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        created_at = d.pop("created_at", UNSET)

        id = d.pop("id", UNSET)

        limit = d.pop("limit", UNSET)

        meter = d.pop("meter", UNSET)

        period = d.pop("period", UNSET)

        updated_at = d.pop("updated_at", UNSET)

        warning_percent = d.pop("warning_percent", UNSET)

        plan_limit = cls(
            created_at=created_at,
            id=id,
            limit=limit,
            meter=meter,
            period=period,
            updated_at=updated_at,
            warning_percent=warning_percent,
        )

        plan_limit.additional_properties = d
        return plan_limit

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
