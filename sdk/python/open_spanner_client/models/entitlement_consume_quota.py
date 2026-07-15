from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="EntitlementConsumeQuota")


@_attrs_define
class EntitlementConsumeQuota:
    """
    Attributes:
        allowed (bool | Unset):
        current (float | Unset):
        enforcement (str | Unset):
        failure_policy (str | Unset):
        from_ (str | Unset):
        limit (float | Unset):
        message (str | Unset):
        meter (str | Unset):
        overage (float | Unset):
        period (str | Unset):
        period_reset_at (str | Unset):
        plan_id (str | Unset):
        plan_name (str | Unset):
        projected (float | Unset):
        quantity (float | Unset):
        remaining (float | Unset):
        retry_after_seconds (int | Unset):
        state (str | Unset):
        subject (str | Unset):
        to (str | Unset):
    """

    allowed: bool | Unset = UNSET
    current: float | Unset = UNSET
    enforcement: str | Unset = UNSET
    failure_policy: str | Unset = UNSET
    from_: str | Unset = UNSET
    limit: float | Unset = UNSET
    message: str | Unset = UNSET
    meter: str | Unset = UNSET
    overage: float | Unset = UNSET
    period: str | Unset = UNSET
    period_reset_at: str | Unset = UNSET
    plan_id: str | Unset = UNSET
    plan_name: str | Unset = UNSET
    projected: float | Unset = UNSET
    quantity: float | Unset = UNSET
    remaining: float | Unset = UNSET
    retry_after_seconds: int | Unset = UNSET
    state: str | Unset = UNSET
    subject: str | Unset = UNSET
    to: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        allowed = self.allowed

        current = self.current

        enforcement = self.enforcement

        failure_policy = self.failure_policy

        from_ = self.from_

        limit = self.limit

        message = self.message

        meter = self.meter

        overage = self.overage

        period = self.period

        period_reset_at = self.period_reset_at

        plan_id = self.plan_id

        plan_name = self.plan_name

        projected = self.projected

        quantity = self.quantity

        remaining = self.remaining

        retry_after_seconds = self.retry_after_seconds

        state = self.state

        subject = self.subject

        to = self.to

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if allowed is not UNSET:
            field_dict["allowed"] = allowed
        if current is not UNSET:
            field_dict["current"] = current
        if enforcement is not UNSET:
            field_dict["enforcement"] = enforcement
        if failure_policy is not UNSET:
            field_dict["failure_policy"] = failure_policy
        if from_ is not UNSET:
            field_dict["from"] = from_
        if limit is not UNSET:
            field_dict["limit"] = limit
        if message is not UNSET:
            field_dict["message"] = message
        if meter is not UNSET:
            field_dict["meter"] = meter
        if overage is not UNSET:
            field_dict["overage"] = overage
        if period is not UNSET:
            field_dict["period"] = period
        if period_reset_at is not UNSET:
            field_dict["period_reset_at"] = period_reset_at
        if plan_id is not UNSET:
            field_dict["plan_id"] = plan_id
        if plan_name is not UNSET:
            field_dict["plan_name"] = plan_name
        if projected is not UNSET:
            field_dict["projected"] = projected
        if quantity is not UNSET:
            field_dict["quantity"] = quantity
        if remaining is not UNSET:
            field_dict["remaining"] = remaining
        if retry_after_seconds is not UNSET:
            field_dict["retry_after_seconds"] = retry_after_seconds
        if state is not UNSET:
            field_dict["state"] = state
        if subject is not UNSET:
            field_dict["subject"] = subject
        if to is not UNSET:
            field_dict["to"] = to

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        allowed = d.pop("allowed", UNSET)

        current = d.pop("current", UNSET)

        enforcement = d.pop("enforcement", UNSET)

        failure_policy = d.pop("failure_policy", UNSET)

        from_ = d.pop("from", UNSET)

        limit = d.pop("limit", UNSET)

        message = d.pop("message", UNSET)

        meter = d.pop("meter", UNSET)

        overage = d.pop("overage", UNSET)

        period = d.pop("period", UNSET)

        period_reset_at = d.pop("period_reset_at", UNSET)

        plan_id = d.pop("plan_id", UNSET)

        plan_name = d.pop("plan_name", UNSET)

        projected = d.pop("projected", UNSET)

        quantity = d.pop("quantity", UNSET)

        remaining = d.pop("remaining", UNSET)

        retry_after_seconds = d.pop("retry_after_seconds", UNSET)

        state = d.pop("state", UNSET)

        subject = d.pop("subject", UNSET)

        to = d.pop("to", UNSET)

        entitlement_consume_quota = cls(
            allowed=allowed,
            current=current,
            enforcement=enforcement,
            failure_policy=failure_policy,
            from_=from_,
            limit=limit,
            message=message,
            meter=meter,
            overage=overage,
            period=period,
            period_reset_at=period_reset_at,
            plan_id=plan_id,
            plan_name=plan_name,
            projected=projected,
            quantity=quantity,
            remaining=remaining,
            retry_after_seconds=retry_after_seconds,
            state=state,
            subject=subject,
            to=to,
        )

        entitlement_consume_quota.additional_properties = d
        return entitlement_consume_quota

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
