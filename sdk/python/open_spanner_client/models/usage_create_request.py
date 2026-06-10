from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.usage_create_request_metadata import UsageCreateRequestMetadata


T = TypeVar("T", bound="UsageCreateRequest")


@_attrs_define
class UsageCreateRequest:
    """
    Attributes:
        idempotency_key (str | Unset): IdempotencyKey replays the original accepted event when reused.
        metadata (UsageCreateRequestMetadata | Unset):
        meter (str | Unset):
        quantity (float | Unset):
        subject (str | Unset):
        timestamp (str | Unset):
    """

    idempotency_key: str | Unset = UNSET
    metadata: UsageCreateRequestMetadata | Unset = UNSET
    meter: str | Unset = UNSET
    quantity: float | Unset = UNSET
    subject: str | Unset = UNSET
    timestamp: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        idempotency_key = self.idempotency_key

        metadata: dict[str, Any] | Unset = UNSET
        if not isinstance(self.metadata, Unset):
            metadata = self.metadata.to_dict()

        meter = self.meter

        quantity = self.quantity

        subject = self.subject

        timestamp = self.timestamp

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if idempotency_key is not UNSET:
            field_dict["idempotency_key"] = idempotency_key
        if metadata is not UNSET:
            field_dict["metadata"] = metadata
        if meter is not UNSET:
            field_dict["meter"] = meter
        if quantity is not UNSET:
            field_dict["quantity"] = quantity
        if subject is not UNSET:
            field_dict["subject"] = subject
        if timestamp is not UNSET:
            field_dict["timestamp"] = timestamp

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.usage_create_request_metadata import UsageCreateRequestMetadata

        d = dict(src_dict)
        idempotency_key = d.pop("idempotency_key", UNSET)

        _metadata = d.pop("metadata", UNSET)
        metadata: UsageCreateRequestMetadata | Unset
        if isinstance(_metadata, Unset):
            metadata = UNSET
        else:
            metadata = UsageCreateRequestMetadata.from_dict(_metadata)

        meter = d.pop("meter", UNSET)

        quantity = d.pop("quantity", UNSET)

        subject = d.pop("subject", UNSET)

        timestamp = d.pop("timestamp", UNSET)

        usage_create_request = cls(
            idempotency_key=idempotency_key,
            metadata=metadata,
            meter=meter,
            quantity=quantity,
            subject=subject,
            timestamp=timestamp,
        )

        usage_create_request.additional_properties = d
        return usage_create_request

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
