from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.usage_bulk_failure import UsageBulkFailure
    from ..models.usage_event import UsageEvent


T = TypeVar("T", bound="UsageBulkResult")


@_attrs_define
class UsageBulkResult:
    """
    Attributes:
        accepted (int | Unset):
        accepted_items (list[UsageEvent] | Unset):
        duplicate_items (list[UsageEvent] | Unset):
        duplicates (int | Unset):
        failed (int | Unset):
        failed_items (list[UsageBulkFailure] | Unset):
    """

    accepted: int | Unset = UNSET
    accepted_items: list[UsageEvent] | Unset = UNSET
    duplicate_items: list[UsageEvent] | Unset = UNSET
    duplicates: int | Unset = UNSET
    failed: int | Unset = UNSET
    failed_items: list[UsageBulkFailure] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        accepted = self.accepted

        accepted_items: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.accepted_items, Unset):
            accepted_items = []
            for accepted_items_item_data in self.accepted_items:
                accepted_items_item = accepted_items_item_data.to_dict()
                accepted_items.append(accepted_items_item)

        duplicate_items: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.duplicate_items, Unset):
            duplicate_items = []
            for duplicate_items_item_data in self.duplicate_items:
                duplicate_items_item = duplicate_items_item_data.to_dict()
                duplicate_items.append(duplicate_items_item)

        duplicates = self.duplicates

        failed = self.failed

        failed_items: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.failed_items, Unset):
            failed_items = []
            for failed_items_item_data in self.failed_items:
                failed_items_item = failed_items_item_data.to_dict()
                failed_items.append(failed_items_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if accepted is not UNSET:
            field_dict["accepted"] = accepted
        if accepted_items is not UNSET:
            field_dict["accepted_items"] = accepted_items
        if duplicate_items is not UNSET:
            field_dict["duplicate_items"] = duplicate_items
        if duplicates is not UNSET:
            field_dict["duplicates"] = duplicates
        if failed is not UNSET:
            field_dict["failed"] = failed
        if failed_items is not UNSET:
            field_dict["failed_items"] = failed_items

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.usage_bulk_failure import UsageBulkFailure
        from ..models.usage_event import UsageEvent

        d = dict(src_dict)
        accepted = d.pop("accepted", UNSET)

        _accepted_items = d.pop("accepted_items", UNSET)
        accepted_items: list[UsageEvent] | Unset = UNSET
        if _accepted_items is not UNSET:
            accepted_items = []
            for accepted_items_item_data in _accepted_items:
                accepted_items_item = UsageEvent.from_dict(accepted_items_item_data)

                accepted_items.append(accepted_items_item)

        _duplicate_items = d.pop("duplicate_items", UNSET)
        duplicate_items: list[UsageEvent] | Unset = UNSET
        if _duplicate_items is not UNSET:
            duplicate_items = []
            for duplicate_items_item_data in _duplicate_items:
                duplicate_items_item = UsageEvent.from_dict(duplicate_items_item_data)

                duplicate_items.append(duplicate_items_item)

        duplicates = d.pop("duplicates", UNSET)

        failed = d.pop("failed", UNSET)

        _failed_items = d.pop("failed_items", UNSET)
        failed_items: list[UsageBulkFailure] | Unset = UNSET
        if _failed_items is not UNSET:
            failed_items = []
            for failed_items_item_data in _failed_items:
                failed_items_item = UsageBulkFailure.from_dict(failed_items_item_data)

                failed_items.append(failed_items_item)

        usage_bulk_result = cls(
            accepted=accepted,
            accepted_items=accepted_items,
            duplicate_items=duplicate_items,
            duplicates=duplicates,
            failed=failed,
            failed_items=failed_items,
        )

        usage_bulk_result.additional_properties = d
        return usage_bulk_result

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
