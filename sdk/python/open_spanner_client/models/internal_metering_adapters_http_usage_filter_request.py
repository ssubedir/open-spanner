from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="InternalMeteringAdaptersHttpUsageFilterRequest")


@_attrs_define
class InternalMeteringAdaptersHttpUsageFilterRequest:
    """
    Attributes:
        field (str | Unset):
        op (str | Unset):
        rules (list[InternalMeteringAdaptersHttpUsageFilterRequest] | Unset):
        type_ (str | Unset):
        value (Any | Unset):
    """

    field: str | Unset = UNSET
    op: str | Unset = UNSET
    rules: list[InternalMeteringAdaptersHttpUsageFilterRequest] | Unset = UNSET
    type_: str | Unset = UNSET
    value: Any | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        field = self.field

        op = self.op

        rules: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.rules, Unset):
            rules = []
            for rules_item_data in self.rules:
                rules_item = rules_item_data.to_dict()
                rules.append(rules_item)

        type_ = self.type_

        value = self.value

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if field is not UNSET:
            field_dict["field"] = field
        if op is not UNSET:
            field_dict["op"] = op
        if rules is not UNSET:
            field_dict["rules"] = rules
        if type_ is not UNSET:
            field_dict["type"] = type_
        if value is not UNSET:
            field_dict["value"] = value

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        field = d.pop("field", UNSET)

        op = d.pop("op", UNSET)

        _rules = d.pop("rules", UNSET)
        rules: list[InternalMeteringAdaptersHttpUsageFilterRequest] | Unset = UNSET
        if _rules is not UNSET:
            rules = []
            for rules_item_data in _rules:
                rules_item = InternalMeteringAdaptersHttpUsageFilterRequest.from_dict(rules_item_data)

                rules.append(rules_item)

        type_ = d.pop("type", UNSET)

        value = d.pop("value", UNSET)

        internal_metering_adapters_http_usage_filter_request = cls(
            field=field,
            op=op,
            rules=rules,
            type_=type_,
            value=value,
        )

        internal_metering_adapters_http_usage_filter_request.additional_properties = d
        return internal_metering_adapters_http_usage_filter_request

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
