from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.usage_prune_meter import UsagePruneMeter


T = TypeVar("T", bound="UsagePruneRun")


@_attrs_define
class UsagePruneRun:
    """
    Attributes:
        created_at (str | Unset):
        deleted (int | Unset):
        dry_run (bool | Unset):
        id (str | Unset):
        meters (list[UsagePruneMeter] | Unset):
    """

    created_at: str | Unset = UNSET
    deleted: int | Unset = UNSET
    dry_run: bool | Unset = UNSET
    id: str | Unset = UNSET
    meters: list[UsagePruneMeter] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        created_at = self.created_at

        deleted = self.deleted

        dry_run = self.dry_run

        id = self.id

        meters: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.meters, Unset):
            meters = []
            for meters_item_data in self.meters:
                meters_item = meters_item_data.to_dict()
                meters.append(meters_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if created_at is not UNSET:
            field_dict["created_at"] = created_at
        if deleted is not UNSET:
            field_dict["deleted"] = deleted
        if dry_run is not UNSET:
            field_dict["dry_run"] = dry_run
        if id is not UNSET:
            field_dict["id"] = id
        if meters is not UNSET:
            field_dict["meters"] = meters

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.usage_prune_meter import UsagePruneMeter

        d = dict(src_dict)
        created_at = d.pop("created_at", UNSET)

        deleted = d.pop("deleted", UNSET)

        dry_run = d.pop("dry_run", UNSET)

        id = d.pop("id", UNSET)

        _meters = d.pop("meters", UNSET)
        meters: list[UsagePruneMeter] | Unset = UNSET
        if _meters is not UNSET:
            meters = []
            for meters_item_data in _meters:
                meters_item = UsagePruneMeter.from_dict(meters_item_data)

                meters.append(meters_item)

        usage_prune_run = cls(
            created_at=created_at,
            deleted=deleted,
            dry_run=dry_run,
            id=id,
            meters=meters,
        )

        usage_prune_run.additional_properties = d
        return usage_prune_run

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
