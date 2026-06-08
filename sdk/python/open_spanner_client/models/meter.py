from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.meter_metadata_schema import MeterMetadataSchema


T = TypeVar("T", bound="Meter")


@_attrs_define
class Meter:
    """
    Attributes:
        aggregation (str | Unset):
        created_at (str | Unset):
        description (str | Unset):
        event_retention_days (int | Unset):
        id (str | Unset):
        metadata_schema (MeterMetadataSchema | Unset):
        name (str | Unset):
        unit (str | Unset):
    """

    aggregation: str | Unset = UNSET
    created_at: str | Unset = UNSET
    description: str | Unset = UNSET
    event_retention_days: int | Unset = UNSET
    id: str | Unset = UNSET
    metadata_schema: MeterMetadataSchema | Unset = UNSET
    name: str | Unset = UNSET
    unit: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        aggregation = self.aggregation

        created_at = self.created_at

        description = self.description

        event_retention_days = self.event_retention_days

        id = self.id

        metadata_schema: dict[str, Any] | Unset = UNSET
        if not isinstance(self.metadata_schema, Unset):
            metadata_schema = self.metadata_schema.to_dict()

        name = self.name

        unit = self.unit

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if aggregation is not UNSET:
            field_dict["aggregation"] = aggregation
        if created_at is not UNSET:
            field_dict["created_at"] = created_at
        if description is not UNSET:
            field_dict["description"] = description
        if event_retention_days is not UNSET:
            field_dict["event_retention_days"] = event_retention_days
        if id is not UNSET:
            field_dict["id"] = id
        if metadata_schema is not UNSET:
            field_dict["metadata_schema"] = metadata_schema
        if name is not UNSET:
            field_dict["name"] = name
        if unit is not UNSET:
            field_dict["unit"] = unit

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.meter_metadata_schema import MeterMetadataSchema

        d = dict(src_dict)
        aggregation = d.pop("aggregation", UNSET)

        created_at = d.pop("created_at", UNSET)

        description = d.pop("description", UNSET)

        event_retention_days = d.pop("event_retention_days", UNSET)

        id = d.pop("id", UNSET)

        _metadata_schema = d.pop("metadata_schema", UNSET)
        metadata_schema: MeterMetadataSchema | Unset
        if isinstance(_metadata_schema, Unset):
            metadata_schema = UNSET
        else:
            metadata_schema = MeterMetadataSchema.from_dict(_metadata_schema)

        name = d.pop("name", UNSET)

        unit = d.pop("unit", UNSET)

        meter = cls(
            aggregation=aggregation,
            created_at=created_at,
            description=description,
            event_retention_days=event_retention_days,
            id=id,
            metadata_schema=metadata_schema,
            name=name,
            unit=unit,
        )

        meter.additional_properties = d
        return meter

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
