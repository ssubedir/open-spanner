from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="EntitlementProgressItem")


@_attrs_define
class EntitlementProgressItem:
    """
    Attributes:
        aggregation (str | Unset):
        current (float | Unset):
        from_ (str | Unset):
        limit (float | Unset):
        meter (str | Unset):
        overage (float | Unset):
        percent (float | Unset):
        period (str | Unset):
        period_reset_at (str | Unset):
        remaining (float | Unset):
        state (str | Unset):
        to (str | Unset):
        unit (str | Unset):
        warning_percent (float | Unset):
    """

    aggregation: str | Unset = UNSET
    current: float | Unset = UNSET
    from_: str | Unset = UNSET
    limit: float | Unset = UNSET
    meter: str | Unset = UNSET
    overage: float | Unset = UNSET
    percent: float | Unset = UNSET
    period: str | Unset = UNSET
    period_reset_at: str | Unset = UNSET
    remaining: float | Unset = UNSET
    state: str | Unset = UNSET
    to: str | Unset = UNSET
    unit: str | Unset = UNSET
    warning_percent: float | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        aggregation = self.aggregation

        current = self.current

        from_ = self.from_

        limit = self.limit

        meter = self.meter

        overage = self.overage

        percent = self.percent

        period = self.period

        period_reset_at = self.period_reset_at

        remaining = self.remaining

        state = self.state

        to = self.to

        unit = self.unit

        warning_percent = self.warning_percent

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if aggregation is not UNSET:
            field_dict["aggregation"] = aggregation
        if current is not UNSET:
            field_dict["current"] = current
        if from_ is not UNSET:
            field_dict["from"] = from_
        if limit is not UNSET:
            field_dict["limit"] = limit
        if meter is not UNSET:
            field_dict["meter"] = meter
        if overage is not UNSET:
            field_dict["overage"] = overage
        if percent is not UNSET:
            field_dict["percent"] = percent
        if period is not UNSET:
            field_dict["period"] = period
        if period_reset_at is not UNSET:
            field_dict["period_reset_at"] = period_reset_at
        if remaining is not UNSET:
            field_dict["remaining"] = remaining
        if state is not UNSET:
            field_dict["state"] = state
        if to is not UNSET:
            field_dict["to"] = to
        if unit is not UNSET:
            field_dict["unit"] = unit
        if warning_percent is not UNSET:
            field_dict["warning_percent"] = warning_percent

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        aggregation = d.pop("aggregation", UNSET)

        current = d.pop("current", UNSET)

        from_ = d.pop("from", UNSET)

        limit = d.pop("limit", UNSET)

        meter = d.pop("meter", UNSET)

        overage = d.pop("overage", UNSET)

        percent = d.pop("percent", UNSET)

        period = d.pop("period", UNSET)

        period_reset_at = d.pop("period_reset_at", UNSET)

        remaining = d.pop("remaining", UNSET)

        state = d.pop("state", UNSET)

        to = d.pop("to", UNSET)

        unit = d.pop("unit", UNSET)

        warning_percent = d.pop("warning_percent", UNSET)

        entitlement_progress_item = cls(
            aggregation=aggregation,
            current=current,
            from_=from_,
            limit=limit,
            meter=meter,
            overage=overage,
            percent=percent,
            period=period,
            period_reset_at=period_reset_at,
            remaining=remaining,
            state=state,
            to=to,
            unit=unit,
            warning_percent=warning_percent,
        )

        entitlement_progress_item.additional_properties = d
        return entitlement_progress_item

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
