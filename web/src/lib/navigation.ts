import {
  BellRing,
  ChartNoAxesCombined,
  CircleGauge,
  FileDown,
  Gauge,
  KeyRound,
  Settings,
  Layers3,
  ListChecks,
  ScanSearch,
  UsersRound,
} from "lucide-react";

export const navigation = [
  {
    label: "Workspace",
    items: [
      { label: "Overview", href: "/overview", icon: CircleGauge },
    ],
  },
  {
    label: "Metering",
    items: [
      { label: "Meters", href: "/meters", icon: Gauge },
      { label: "Subjects", href: "/subjects", icon: UsersRound },
      { label: "Plans", href: "/plans", icon: Layers3 },
      { label: "Usage", href: "/usage", icon: ChartNoAxesCombined },
      { label: "Decisions", href: "/decisions", icon: ListChecks },
    ],
  },
  {
    label: "Operations",
    items: [
      { label: "Alerts", href: "/alerts", icon: BellRing },
      { label: "Exports", href: "/exports", icon: FileDown },
      { label: "Reconciliation", href: "/reconciliation", icon: ScanSearch },
    ],
  },
  {
    label: "Access",
    items: [
      { label: "API keys", href: "/api-keys", icon: KeyRound },
      { label: "Workspace", href: "/settings/workspace", icon: Settings },
    ],
  },
];
