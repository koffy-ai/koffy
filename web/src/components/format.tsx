import { Tag } from "antd";

export function coins(value?: number) {
  return (value ?? 0).toLocaleString("zh-CN");
}

export function points(value?: number) {
  return `${coins(value)}点`;
}

export function cents(value?: number) {
  return `¥${((value ?? 0) / 100).toFixed(2)}`;
}

export function time(value?: string) {
  if (!value) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(new Date(value));
}

export function date(value?: string) {
  if (!value) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit"
  }).format(new Date(value));
}

const statusColors: Record<string, string> = {
  active: "green",
  paid: "green",
  committed: "green",
  authorized: "blue",
  pending: "gold",
  cancelled: "default",
  canceled: "default",
  failed: "red",
  disabled: "default",
  expired: "default",
  hybrid: "blue",
  coins: "purple",
  entitlement: "cyan"
};

export function StatusTag({ value }: { value?: string }) {
  if (!value) return <Tag>-</Tag>;
  return <Tag color={statusColors[value] || "default"}>{value}</Tag>;
}
