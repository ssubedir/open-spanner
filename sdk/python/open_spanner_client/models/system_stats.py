from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.system_last_prune_run import SystemLastPruneRun


T = TypeVar("T", bound="SystemStats")


@_attrs_define
class SystemStats:
    """
    Attributes:
        last_prune_run (SystemLastPruneRun | Unset):
        meters (int | Unset):
        prune_runs (int | Unset):
        usage_events (int | Unset):
    """

    last_prune_run: SystemLastPruneRun | Unset = UNSET
    meters: int | Unset = UNSET
    prune_runs: int | Unset = UNSET
    usage_events: int | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        last_prune_run: dict[str, Any] | Unset = UNSET
        if not isinstance(self.last_prune_run, Unset):
            last_prune_run = self.last_prune_run.to_dict()

        meters = self.meters

        prune_runs = self.prune_runs

        usage_events = self.usage_events

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if last_prune_run is not UNSET:
            field_dict["last_prune_run"] = last_prune_run
        if meters is not UNSET:
            field_dict["meters"] = meters
        if prune_runs is not UNSET:
            field_dict["prune_runs"] = prune_runs
        if usage_events is not UNSET:
            field_dict["usage_events"] = usage_events

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.system_last_prune_run import SystemLastPruneRun

        d = dict(src_dict)
        _last_prune_run = d.pop("last_prune_run", UNSET)
        last_prune_run: SystemLastPruneRun | Unset
        if isinstance(_last_prune_run, Unset):
            last_prune_run = UNSET
        else:
            last_prune_run = SystemLastPruneRun.from_dict(_last_prune_run)

        meters = d.pop("meters", UNSET)

        prune_runs = d.pop("prune_runs", UNSET)

        usage_events = d.pop("usage_events", UNSET)

        system_stats = cls(
            last_prune_run=last_prune_run,
            meters=meters,
            prune_runs=prune_runs,
            usage_events=usage_events,
        )

        system_stats.additional_properties = d
        return system_stats

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
