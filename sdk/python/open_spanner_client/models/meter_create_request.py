from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.meter_create_request_metadata_schema import MeterCreateRequestMetadataSchema
    from ..models.meter_dimension_request import MeterDimensionRequest


T = TypeVar("T", bound="MeterCreateRequest")


@_attrs_define
class MeterCreateRequest:
    """
    Attributes:
        aggregation (str | Unset):
        description (str | Unset):
        dimensions (list[MeterDimensionRequest] | Unset):
        event_retention_days (int | Unset):
        metadata_schema (MeterCreateRequestMetadataSchema | Unset):
        name (str | Unset):
        unit (str | Unset):
    """

    aggregation: str | Unset = UNSET
    description: str | Unset = UNSET
    dimensions: list[MeterDimensionRequest] | Unset = UNSET
    event_retention_days: int | Unset = UNSET
    metadata_schema: MeterCreateRequestMetadataSchema | Unset = UNSET
    name: str | Unset = UNSET
    unit: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        aggregation = self.aggregation

        description = self.description

        dimensions: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.dimensions, Unset):
            dimensions = []
            for dimensions_item_data in self.dimensions:
                dimensions_item = dimensions_item_data.to_dict()
                dimensions.append(dimensions_item)

        event_retention_days = self.event_retention_days

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
        if description is not UNSET:
            field_dict["description"] = description
        if dimensions is not UNSET:
            field_dict["dimensions"] = dimensions
        if event_retention_days is not UNSET:
            field_dict["event_retention_days"] = event_retention_days
        if metadata_schema is not UNSET:
            field_dict["metadata_schema"] = metadata_schema
        if name is not UNSET:
            field_dict["name"] = name
        if unit is not UNSET:
            field_dict["unit"] = unit

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.meter_create_request_metadata_schema import MeterCreateRequestMetadataSchema
        from ..models.meter_dimension_request import MeterDimensionRequest

        d = dict(src_dict)
        aggregation = d.pop("aggregation", UNSET)

        description = d.pop("description", UNSET)

        _dimensions = d.pop("dimensions", UNSET)
        dimensions: list[MeterDimensionRequest] | Unset = UNSET
        if _dimensions is not UNSET:
            dimensions = []
            for dimensions_item_data in _dimensions:
                dimensions_item = MeterDimensionRequest.from_dict(dimensions_item_data)

                dimensions.append(dimensions_item)

        event_retention_days = d.pop("event_retention_days", UNSET)

        _metadata_schema = d.pop("metadata_schema", UNSET)
        metadata_schema: MeterCreateRequestMetadataSchema | Unset
        if isinstance(_metadata_schema, Unset):
            metadata_schema = UNSET
        else:
            metadata_schema = MeterCreateRequestMetadataSchema.from_dict(_metadata_schema)

        name = d.pop("name", UNSET)

        unit = d.pop("unit", UNSET)

        meter_create_request = cls(
            aggregation=aggregation,
            description=description,
            dimensions=dimensions,
            event_retention_days=event_retention_days,
            metadata_schema=metadata_schema,
            name=name,
            unit=unit,
        )

        meter_create_request.additional_properties = d
        return meter_create_request

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
