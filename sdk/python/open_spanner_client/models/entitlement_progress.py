from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.entitlement_progress_item import EntitlementProgressItem
    from ..models.plan import Plan


T = TypeVar("T", bound="EntitlementProgress")


@_attrs_define
class EntitlementProgress:
    """
    Attributes:
        items (list[EntitlementProgressItem] | Unset):
        plan (Plan | Unset):
        subject (str | Unset):
    """

    items: list[EntitlementProgressItem] | Unset = UNSET
    plan: Plan | Unset = UNSET
    subject: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        items: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.items, Unset):
            items = []
            for items_item_data in self.items:
                items_item = items_item_data.to_dict()
                items.append(items_item)

        plan: dict[str, Any] | Unset = UNSET
        if not isinstance(self.plan, Unset):
            plan = self.plan.to_dict()

        subject = self.subject

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if items is not UNSET:
            field_dict["items"] = items
        if plan is not UNSET:
            field_dict["plan"] = plan
        if subject is not UNSET:
            field_dict["subject"] = subject

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.entitlement_progress_item import EntitlementProgressItem
        from ..models.plan import Plan

        d = dict(src_dict)
        _items = d.pop("items", UNSET)
        items: list[EntitlementProgressItem] | Unset = UNSET
        if _items is not UNSET:
            items = []
            for items_item_data in _items:
                items_item = EntitlementProgressItem.from_dict(items_item_data)

                items.append(items_item)

        _plan = d.pop("plan", UNSET)
        plan: Plan | Unset
        if isinstance(_plan, Unset):
            plan = UNSET
        else:
            plan = Plan.from_dict(_plan)

        subject = d.pop("subject", UNSET)

        entitlement_progress = cls(
            items=items,
            plan=plan,
            subject=subject,
        )

        entitlement_progress.additional_properties = d
        return entitlement_progress

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
