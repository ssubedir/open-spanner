from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.usage_event_metadata import UsageEventMetadata


T = TypeVar("T", bound="UsageEvent")


@_attrs_define
class UsageEvent:
    """
    Attributes:
        id (str | Unset):
        idempotency_key (str | Unset):
        metadata (UsageEventMetadata | Unset):
        meter (str | Unset):
        quantity (float | Unset):
        received_at (str | Unset):
        subject (str | Unset):
        timestamp (str | Unset):
    """

    id: str | Unset = UNSET
    idempotency_key: str | Unset = UNSET
    metadata: UsageEventMetadata | Unset = UNSET
    meter: str | Unset = UNSET
    quantity: float | Unset = UNSET
    received_at: str | Unset = UNSET
    subject: str | Unset = UNSET
    timestamp: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = self.id

        idempotency_key = self.idempotency_key

        metadata: dict[str, Any] | Unset = UNSET
        if not isinstance(self.metadata, Unset):
            metadata = self.metadata.to_dict()

        meter = self.meter

        quantity = self.quantity

        received_at = self.received_at

        subject = self.subject

        timestamp = self.timestamp

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if id is not UNSET:
            field_dict["id"] = id
        if idempotency_key is not UNSET:
            field_dict["idempotency_key"] = idempotency_key
        if metadata is not UNSET:
            field_dict["metadata"] = metadata
        if meter is not UNSET:
            field_dict["meter"] = meter
        if quantity is not UNSET:
            field_dict["quantity"] = quantity
        if received_at is not UNSET:
            field_dict["received_at"] = received_at
        if subject is not UNSET:
            field_dict["subject"] = subject
        if timestamp is not UNSET:
            field_dict["timestamp"] = timestamp

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.usage_event_metadata import UsageEventMetadata

        d = dict(src_dict)
        id = d.pop("id", UNSET)

        idempotency_key = d.pop("idempotency_key", UNSET)

        _metadata = d.pop("metadata", UNSET)
        metadata: UsageEventMetadata | Unset
        if isinstance(_metadata, Unset):
            metadata = UNSET
        else:
            metadata = UsageEventMetadata.from_dict(_metadata)

        meter = d.pop("meter", UNSET)

        quantity = d.pop("quantity", UNSET)

        received_at = d.pop("received_at", UNSET)

        subject = d.pop("subject", UNSET)

        timestamp = d.pop("timestamp", UNSET)

        usage_event = cls(
            id=id,
            idempotency_key=idempotency_key,
            metadata=metadata,
            meter=meter,
            quantity=quantity,
            received_at=received_at,
            subject=subject,
            timestamp=timestamp,
        )

        usage_event.additional_properties = d
        return usage_event

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
