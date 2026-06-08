from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="UsageIngestionRun")


@_attrs_define
class UsageIngestionRun:
    """
    Attributes:
        accepted (int | Unset):
        created_at (str | Unset):
        duplicates (int | Unset):
        failed (int | Unset):
        id (str | Unset):
        kind (str | Unset):
    """

    accepted: int | Unset = UNSET
    created_at: str | Unset = UNSET
    duplicates: int | Unset = UNSET
    failed: int | Unset = UNSET
    id: str | Unset = UNSET
    kind: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        accepted = self.accepted

        created_at = self.created_at

        duplicates = self.duplicates

        failed = self.failed

        id = self.id

        kind = self.kind

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if accepted is not UNSET:
            field_dict["accepted"] = accepted
        if created_at is not UNSET:
            field_dict["created_at"] = created_at
        if duplicates is not UNSET:
            field_dict["duplicates"] = duplicates
        if failed is not UNSET:
            field_dict["failed"] = failed
        if id is not UNSET:
            field_dict["id"] = id
        if kind is not UNSET:
            field_dict["kind"] = kind

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        accepted = d.pop("accepted", UNSET)

        created_at = d.pop("created_at", UNSET)

        duplicates = d.pop("duplicates", UNSET)

        failed = d.pop("failed", UNSET)

        id = d.pop("id", UNSET)

        kind = d.pop("kind", UNSET)

        usage_ingestion_run = cls(
            accepted=accepted,
            created_at=created_at,
            duplicates=duplicates,
            failed=failed,
            id=id,
            kind=kind,
        )

        usage_ingestion_run.additional_properties = d
        return usage_ingestion_run

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
