from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="EntitlementState")


@_attrs_define
class EntitlementState:
    """
    Attributes:
        current (float | Unset):
        evaluated_at (str | Unset):
        limit (float | Unset):
        message (str | Unset):
        meter (str | Unset):
        period (str | Unset):
        plan_id (str | Unset):
        plan_name (str | Unset):
        remaining (float | Unset):
        state (str | Unset):
        subject (str | Unset):
        updated_at (str | Unset):
        warning_percent (float | Unset):
    """

    current: float | Unset = UNSET
    evaluated_at: str | Unset = UNSET
    limit: float | Unset = UNSET
    message: str | Unset = UNSET
    meter: str | Unset = UNSET
    period: str | Unset = UNSET
    plan_id: str | Unset = UNSET
    plan_name: str | Unset = UNSET
    remaining: float | Unset = UNSET
    state: str | Unset = UNSET
    subject: str | Unset = UNSET
    updated_at: str | Unset = UNSET
    warning_percent: float | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        current = self.current

        evaluated_at = self.evaluated_at

        limit = self.limit

        message = self.message

        meter = self.meter

        period = self.period

        plan_id = self.plan_id

        plan_name = self.plan_name

        remaining = self.remaining

        state = self.state

        subject = self.subject

        updated_at = self.updated_at

        warning_percent = self.warning_percent

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if current is not UNSET:
            field_dict["current"] = current
        if evaluated_at is not UNSET:
            field_dict["evaluated_at"] = evaluated_at
        if limit is not UNSET:
            field_dict["limit"] = limit
        if message is not UNSET:
            field_dict["message"] = message
        if meter is not UNSET:
            field_dict["meter"] = meter
        if period is not UNSET:
            field_dict["period"] = period
        if plan_id is not UNSET:
            field_dict["plan_id"] = plan_id
        if plan_name is not UNSET:
            field_dict["plan_name"] = plan_name
        if remaining is not UNSET:
            field_dict["remaining"] = remaining
        if state is not UNSET:
            field_dict["state"] = state
        if subject is not UNSET:
            field_dict["subject"] = subject
        if updated_at is not UNSET:
            field_dict["updated_at"] = updated_at
        if warning_percent is not UNSET:
            field_dict["warning_percent"] = warning_percent

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        current = d.pop("current", UNSET)

        evaluated_at = d.pop("evaluated_at", UNSET)

        limit = d.pop("limit", UNSET)

        message = d.pop("message", UNSET)

        meter = d.pop("meter", UNSET)

        period = d.pop("period", UNSET)

        plan_id = d.pop("plan_id", UNSET)

        plan_name = d.pop("plan_name", UNSET)

        remaining = d.pop("remaining", UNSET)

        state = d.pop("state", UNSET)

        subject = d.pop("subject", UNSET)

        updated_at = d.pop("updated_at", UNSET)

        warning_percent = d.pop("warning_percent", UNSET)

        entitlement_state = cls(
            current=current,
            evaluated_at=evaluated_at,
            limit=limit,
            message=message,
            meter=meter,
            period=period,
            plan_id=plan_id,
            plan_name=plan_name,
            remaining=remaining,
            state=state,
            subject=subject,
            updated_at=updated_at,
            warning_percent=warning_percent,
        )

        entitlement_state.additional_properties = d
        return entitlement_state

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
