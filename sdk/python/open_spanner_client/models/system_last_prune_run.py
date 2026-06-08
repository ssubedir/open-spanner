from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="SystemLastPruneRun")


@_attrs_define
class SystemLastPruneRun:
    """
    Attributes:
        created_at (str | Unset):
        deleted (int | Unset):
        dry_run (bool | Unset):
        id (str | Unset):
    """

    created_at: str | Unset = UNSET
    deleted: int | Unset = UNSET
    dry_run: bool | Unset = UNSET
    id: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        created_at = self.created_at

        deleted = self.deleted

        dry_run = self.dry_run

        id = self.id

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

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        created_at = d.pop("created_at", UNSET)

        deleted = d.pop("deleted", UNSET)

        dry_run = d.pop("dry_run", UNSET)

        id = d.pop("id", UNSET)

        system_last_prune_run = cls(
            created_at=created_at,
            deleted=deleted,
            dry_run=dry_run,
            id=id,
        )

        system_last_prune_run.additional_properties = d
        return system_last_prune_run

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
