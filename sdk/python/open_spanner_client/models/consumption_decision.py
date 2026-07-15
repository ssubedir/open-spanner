from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.entitlement_consume_quota import EntitlementConsumeQuota


T = TypeVar("T", bound="ConsumptionDecision")


@_attrs_define
class ConsumptionDecision:
    """
    Attributes:
        accepted (bool | Unset):
        created_at (str | Unset):
        evaluation_failed (bool | Unset):
        event_id (str | Unset):
        idempotency_key (str | Unset):
        quota (EntitlementConsumeQuota | Unset):
    """

    accepted: bool | Unset = UNSET
    created_at: str | Unset = UNSET
    evaluation_failed: bool | Unset = UNSET
    event_id: str | Unset = UNSET
    idempotency_key: str | Unset = UNSET
    quota: EntitlementConsumeQuota | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        accepted = self.accepted

        created_at = self.created_at

        evaluation_failed = self.evaluation_failed

        event_id = self.event_id

        idempotency_key = self.idempotency_key

        quota: dict[str, Any] | Unset = UNSET
        if not isinstance(self.quota, Unset):
            quota = self.quota.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if accepted is not UNSET:
            field_dict["accepted"] = accepted
        if created_at is not UNSET:
            field_dict["created_at"] = created_at
        if evaluation_failed is not UNSET:
            field_dict["evaluation_failed"] = evaluation_failed
        if event_id is not UNSET:
            field_dict["event_id"] = event_id
        if idempotency_key is not UNSET:
            field_dict["idempotency_key"] = idempotency_key
        if quota is not UNSET:
            field_dict["quota"] = quota

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.entitlement_consume_quota import EntitlementConsumeQuota

        d = dict(src_dict)
        accepted = d.pop("accepted", UNSET)

        created_at = d.pop("created_at", UNSET)

        evaluation_failed = d.pop("evaluation_failed", UNSET)

        event_id = d.pop("event_id", UNSET)

        idempotency_key = d.pop("idempotency_key", UNSET)

        _quota = d.pop("quota", UNSET)
        quota: EntitlementConsumeQuota | Unset
        if isinstance(_quota, Unset):
            quota = UNSET
        else:
            quota = EntitlementConsumeQuota.from_dict(_quota)

        consumption_decision = cls(
            accepted=accepted,
            created_at=created_at,
            evaluation_failed=evaluation_failed,
            event_id=event_id,
            idempotency_key=idempotency_key,
            quota=quota,
        )

        consumption_decision.additional_properties = d
        return consumption_decision

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
