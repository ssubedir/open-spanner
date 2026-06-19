from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.meter_dimension import MeterDimension


T = TypeVar("T", bound="Meter")


@_attrs_define
class Meter:
    """
    Attributes:
        aggregation (str | Unset):
        created_at (str | Unset):
        description (str | Unset):
        dimensions (list[MeterDimension] | Unset):
        event_retention_days (int | Unset):
        id (str | Unset):
        name (str | Unset):
        unit (str | Unset):
    """

    aggregation: str | Unset = UNSET
    created_at: str | Unset = UNSET
    description: str | Unset = UNSET
    dimensions: list[MeterDimension] | Unset = UNSET
    event_retention_days: int | Unset = UNSET
    id: str | Unset = UNSET
    name: str | Unset = UNSET
    unit: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        aggregation = self.aggregation

        created_at = self.created_at

        description = self.description

        dimensions: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.dimensions, Unset):
            dimensions = []
            for dimensions_item_data in self.dimensions:
                dimensions_item = dimensions_item_data.to_dict()
                dimensions.append(dimensions_item)

        event_retention_days = self.event_retention_days

        id = self.id

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
        if dimensions is not UNSET:
            field_dict["dimensions"] = dimensions
        if event_retention_days is not UNSET:
            field_dict["event_retention_days"] = event_retention_days
        if id is not UNSET:
            field_dict["id"] = id
        if name is not UNSET:
            field_dict["name"] = name
        if unit is not UNSET:
            field_dict["unit"] = unit

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.meter_dimension import MeterDimension

        d = dict(src_dict)
        aggregation = d.pop("aggregation", UNSET)

        created_at = d.pop("created_at", UNSET)

        description = d.pop("description", UNSET)

        _dimensions = d.pop("dimensions", UNSET)
        dimensions: list[MeterDimension] | Unset = UNSET
        if _dimensions is not UNSET:
            dimensions = []
            for dimensions_item_data in _dimensions:
                dimensions_item = MeterDimension.from_dict(dimensions_item_data)

                dimensions.append(dimensions_item)

        event_retention_days = d.pop("event_retention_days", UNSET)

        id = d.pop("id", UNSET)

        name = d.pop("name", UNSET)

        unit = d.pop("unit", UNSET)

        meter = cls(
            aggregation=aggregation,
            created_at=created_at,
            description=description,
            dimensions=dimensions,
            event_retention_days=event_retention_days,
            id=id,
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
