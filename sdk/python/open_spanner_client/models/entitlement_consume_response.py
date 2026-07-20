from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.entitlement_consume_quota import EntitlementConsumeQuota
    from ..models.usage_event import UsageEvent


T = TypeVar("T", bound="EntitlementConsumeResponse")


@_attrs_define
class EntitlementConsumeResponse:
    """
    Attributes:
        accepted (bool | Unset):
        evaluation_failed (bool | Unset):
        event (UsageEvent | Unset):
        quota (EntitlementConsumeQuota | Unset):
        replayed (bool | Unset):
    """

    accepted: bool | Unset = UNSET
    evaluation_failed: bool | Unset = UNSET
    event: UsageEvent | Unset = UNSET
    quota: EntitlementConsumeQuota | Unset = UNSET
    replayed: bool | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        accepted = self.accepted

        evaluation_failed = self.evaluation_failed

        event: dict[str, Any] | Unset = UNSET
        if not isinstance(self.event, Unset):
            event = self.event.to_dict()

        quota: dict[str, Any] | Unset = UNSET
        if not isinstance(self.quota, Unset):
            quota = self.quota.to_dict()

        replayed = self.replayed

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if accepted is not UNSET:
            field_dict["accepted"] = accepted
        if evaluation_failed is not UNSET:
            field_dict["evaluation_failed"] = evaluation_failed
        if event is not UNSET:
            field_dict["event"] = event
        if quota is not UNSET:
            field_dict["quota"] = quota
        if replayed is not UNSET:
            field_dict["replayed"] = replayed

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.entitlement_consume_quota import EntitlementConsumeQuota
        from ..models.usage_event import UsageEvent

        d = dict(src_dict)
        accepted = d.pop("accepted", UNSET)

        evaluation_failed = d.pop("evaluation_failed", UNSET)

        _event = d.pop("event", UNSET)
        event: UsageEvent | Unset
        if isinstance(_event, Unset):
            event = UNSET
        else:
            event = UsageEvent.from_dict(_event)

        _quota = d.pop("quota", UNSET)
        quota: EntitlementConsumeQuota | Unset
        if isinstance(_quota, Unset):
            quota = UNSET
        else:
            quota = EntitlementConsumeQuota.from_dict(_quota)

        replayed = d.pop("replayed", UNSET)

        entitlement_consume_response = cls(
            accepted=accepted,
            evaluation_failed=evaluation_failed,
            event=event,
            quota=quota,
            replayed=replayed,
        )

        entitlement_consume_response.additional_properties = d
        return entitlement_consume_response

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
