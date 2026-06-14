from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.meter_update_request_metadata_schema import MeterUpdateRequestMetadataSchema


T = TypeVar("T", bound="MeterUpdateRequest")


@_attrs_define
class MeterUpdateRequest:
    """
    Attributes:
        aggregation (str | Unset):
        description (str | Unset):
        event_retention_days (int | Unset):
        metadata_schema (MeterUpdateRequestMetadataSchema | Unset):
        unit (str | Unset):
    """

    aggregation: str | Unset = UNSET
    description: str | Unset = UNSET
    event_retention_days: int | Unset = UNSET
    metadata_schema: MeterUpdateRequestMetadataSchema | Unset = UNSET
    unit: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        aggregation = self.aggregation

        description = self.description

        event_retention_days = self.event_retention_days

        metadata_schema: dict[str, Any] | Unset = UNSET
        if not isinstance(self.metadata_schema, Unset):
            metadata_schema = self.metadata_schema.to_dict()

        unit = self.unit

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if aggregation is not UNSET:
            field_dict["aggregation"] = aggregation
        if description is not UNSET:
            field_dict["description"] = description
        if event_retention_days is not UNSET:
            field_dict["event_retention_days"] = event_retention_days
        if metadata_schema is not UNSET:
            field_dict["metadata_schema"] = metadata_schema
        if unit is not UNSET:
            field_dict["unit"] = unit

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.meter_update_request_metadata_schema import MeterUpdateRequestMetadataSchema

        d = dict(src_dict)
        aggregation = d.pop("aggregation", UNSET)

        description = d.pop("description", UNSET)

        event_retention_days = d.pop("event_retention_days", UNSET)

        _metadata_schema = d.pop("metadata_schema", UNSET)
        metadata_schema: MeterUpdateRequestMetadataSchema | Unset
        if isinstance(_metadata_schema, Unset):
            metadata_schema = UNSET
        else:
            metadata_schema = MeterUpdateRequestMetadataSchema.from_dict(_metadata_schema)

        unit = d.pop("unit", UNSET)

        meter_update_request = cls(
            aggregation=aggregation,
            description=description,
            event_retention_days=event_retention_days,
            metadata_schema=metadata_schema,
            unit=unit,
        )

        meter_update_request.additional_properties = d
        return meter_update_request

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
