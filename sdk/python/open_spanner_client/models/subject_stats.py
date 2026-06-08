from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="SubjectStats")


@_attrs_define
class SubjectStats:
    """
    Attributes:
        last_event_at (str | Unset):
        meters (int | Unset):
        subject (str | Unset):
        usage_events (int | Unset):
    """

    last_event_at: str | Unset = UNSET
    meters: int | Unset = UNSET
    subject: str | Unset = UNSET
    usage_events: int | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        last_event_at = self.last_event_at

        meters = self.meters

        subject = self.subject

        usage_events = self.usage_events

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if last_event_at is not UNSET:
            field_dict["last_event_at"] = last_event_at
        if meters is not UNSET:
            field_dict["meters"] = meters
        if subject is not UNSET:
            field_dict["subject"] = subject
        if usage_events is not UNSET:
            field_dict["usage_events"] = usage_events

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        last_event_at = d.pop("last_event_at", UNSET)

        meters = d.pop("meters", UNSET)

        subject = d.pop("subject", UNSET)

        usage_events = d.pop("usage_events", UNSET)

        subject_stats = cls(
            last_event_at=last_event_at,
            meters=meters,
            subject=subject,
            usage_events=usage_events,
        )

        subject_stats.additional_properties = d
        return subject_stats

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
