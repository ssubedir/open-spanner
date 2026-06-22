from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.plan_limit import PlanLimit


T = TypeVar("T", bound="Plan")


@_attrs_define
class Plan:
    """
    Attributes:
        created_at (str | Unset):
        description (str | Unset):
        id (str | Unset):
        is_current (bool | Unset):
        limits (list[PlanLimit] | Unset):
        name (str | Unset):
        parent_plan_id (str | Unset):
        updated_at (str | Unset):
        version (int | Unset):
    """

    created_at: str | Unset = UNSET
    description: str | Unset = UNSET
    id: str | Unset = UNSET
    is_current: bool | Unset = UNSET
    limits: list[PlanLimit] | Unset = UNSET
    name: str | Unset = UNSET
    parent_plan_id: str | Unset = UNSET
    updated_at: str | Unset = UNSET
    version: int | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        created_at = self.created_at

        description = self.description

        id = self.id

        is_current = self.is_current

        limits: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.limits, Unset):
            limits = []
            for limits_item_data in self.limits:
                limits_item = limits_item_data.to_dict()
                limits.append(limits_item)

        name = self.name

        parent_plan_id = self.parent_plan_id

        updated_at = self.updated_at

        version = self.version

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if created_at is not UNSET:
            field_dict["created_at"] = created_at
        if description is not UNSET:
            field_dict["description"] = description
        if id is not UNSET:
            field_dict["id"] = id
        if is_current is not UNSET:
            field_dict["is_current"] = is_current
        if limits is not UNSET:
            field_dict["limits"] = limits
        if name is not UNSET:
            field_dict["name"] = name
        if parent_plan_id is not UNSET:
            field_dict["parent_plan_id"] = parent_plan_id
        if updated_at is not UNSET:
            field_dict["updated_at"] = updated_at
        if version is not UNSET:
            field_dict["version"] = version

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.plan_limit import PlanLimit

        d = dict(src_dict)
        created_at = d.pop("created_at", UNSET)

        description = d.pop("description", UNSET)

        id = d.pop("id", UNSET)

        is_current = d.pop("is_current", UNSET)

        _limits = d.pop("limits", UNSET)
        limits: list[PlanLimit] | Unset = UNSET
        if _limits is not UNSET:
            limits = []
            for limits_item_data in _limits:
                limits_item = PlanLimit.from_dict(limits_item_data)

                limits.append(limits_item)

        name = d.pop("name", UNSET)

        parent_plan_id = d.pop("parent_plan_id", UNSET)

        updated_at = d.pop("updated_at", UNSET)

        version = d.pop("version", UNSET)

        plan = cls(
            created_at=created_at,
            description=description,
            id=id,
            is_current=is_current,
            limits=limits,
            name=name,
            parent_plan_id=parent_plan_id,
            updated_at=updated_at,
            version=version,
        )

        plan.additional_properties = d
        return plan

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
