import { useQuery } from "@tanstack/react-query";
import { Card, InputNumber, Statistic, Table } from "antd";
import { useState } from "react";
import { api } from "../../api/client";
import type { AdminMetricsSummary } from "../../api/types";
import { coins, cents } from "../../components/format";

type UsageMetric = AdminMetricsSummary["usage_by_app"][number];
type RechargeMetric = AdminMetricsSummary["recharge_by_status"][number];

export function AdminOverviewPage() {
  const [days, setDays] = useState(7);
  const metrics = useQuery({ queryKey: ["admin-metrics", days], queryFn: () => api.adminMetrics(days) });
  const usage = metrics.data?.usage_by_app || [];
  const recharge = metrics.data?.recharge_by_status || [];
  const totals = usage.reduce(
    (acc, item) => ({
      requests: acc.requests + item.request_count,
      coins: acc.coins + item.charged_coins,
      units: acc.units + item.charged_units,
      tokens: acc.tokens + item.actual_total_tokens
    }),
    { requests: 0, coins: 0, units: 0, tokens: 0 }
  );
  const rechargeTotal = recharge.reduce((sum, item) => sum + item.amount_cents, 0);

  return (
    <div className="page-stack">
      <div className="page-title-row">
        <h1 className="page-title">运营概览</h1>
        <InputNumber min={1} max={90} value={days} onChange={(value) => setDays(value || 7)} addonAfter="天" />
      </div>
      <div className="metric-grid">
        <Card className="metric-card"><Statistic title="请求数" value={totals.requests} formatter={(value) => coins(Number(value))} /></Card>
        <Card className="metric-card"><Statistic title="扣点数" value={totals.coins} formatter={(value) => coins(Number(value))} /></Card>
        <Card className="metric-card"><Statistic title="权益消耗" value={totals.units} formatter={(value) => coins(Number(value))} /></Card>
        <Card className="metric-card"><Statistic title="充值金额" value={rechargeTotal} formatter={(value) => cents(Number(value))} /></Card>
      </div>
      <Card title="应用调用" className="table-card">
        <Table<UsageMetric>
          rowKey="app_code"
          size="small"
          loading={metrics.isPending}
          dataSource={usage}
          columns={[
            { title: "应用", dataIndex: "app_code" },
            { title: "请求", render: (_, row) => coins(row.request_count) },
            { title: "成功", render: (_, row) => coins(row.committed_count) },
            { title: "取消", render: (_, row) => coins(row.cancelled_count) },
            { title: "扣点", render: (_, row) => coins(row.charged_coins) },
            { title: "权益", render: (_, row) => coins(row.charged_units) },
            { title: "Tokens", render: (_, row) => coins(row.actual_total_tokens) },
            { title: "图片", render: (_, row) => coins(row.actual_images) },
            { title: "视频秒", render: (_, row) => coins(row.actual_video_seconds) }
          ]}
          pagination={false}
        />
      </Card>
      <Card title="充值汇总" className="table-card">
        <Table<RechargeMetric>
          rowKey="status"
          size="small"
          loading={metrics.isPending}
          dataSource={recharge}
          columns={[
            { title: "状态", dataIndex: "status" },
            { title: "订单数", render: (_, row) => coins(row.order_count) },
            { title: "金额", render: (_, row) => cents(row.amount_cents) },
            { title: "点数", render: (_, row) => coins(row.coins) }
          ]}
          pagination={false}
        />
      </Card>
    </div>
  );
}
