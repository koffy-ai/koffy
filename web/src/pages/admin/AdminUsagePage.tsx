import { useQuery } from "@tanstack/react-query";
import { Button, Card, Form, Input, InputNumber, Space, Table } from "antd";
import { Search } from "lucide-react";
import { useMemo, useState } from "react";
import { api } from "../../api/client";
import type { UsageRequestItem } from "../../api/types";
import { coins, StatusTag, time } from "../../components/format";
import { AdminSelect, AdminUserSearchSelect } from "./AdminSelectors";

export function AdminUsagePage() {
  const [filter, setFilter] = useState({ app_code: "", user_id: "", limit: 100 });
  const apps = useQuery({ queryKey: ["admin-apps"], queryFn: api.adminApps });
  const appOptions = useMemo(
    () => (apps.data?.items || []).map((item) => ({ value: item.app_code, label: item.name })),
    [apps.data?.items]
  );
  const params = useMemo(() => {
    const next = new URLSearchParams();
    if (filter.app_code) next.set("app_code", filter.app_code);
    if (filter.user_id) next.set("user_id", filter.user_id);
    next.set("limit", String(filter.limit));
    return next;
  }, [filter]);
  const usage = useQuery({ queryKey: ["admin-usage", params.toString()], queryFn: () => api.adminUsageRequests(params) });

  return (
    <div className="page-stack">
      <div className="page-title-row">
        <h1 className="page-title">调用记录</h1>
      </div>
      <Card>
        <Form layout="inline" initialValues={filter} onFinish={(values) => setFilter(values)}>
          <Form.Item label="App Code" name="app_code"><AdminSelect options={appOptions} placeholder="选择应用" width={180} /></Form.Item>
          <Form.Item label="用户" name="user_id"><AdminUserSearchSelect width={320} /></Form.Item>
          <Form.Item label="数量" name="limit"><InputNumber min={1} max={200} /></Form.Item>
          <Button type="primary" htmlType="submit" icon={<Search size={16} />}>查询</Button>
        </Form>
      </Card>
      <Card className="table-card">
        <Table<UsageRequestItem>
          rowKey="id"
          size="small"
          loading={usage.isPending}
          dataSource={usage.data?.items || []}
          scroll={{ x: 1500 }}
          columns={[
            { title: "时间", fixed: "left", width: 120, render: (_, row) => time(row.created_at) },
            { title: "应用", dataIndex: "app_code", width: 130 },
            { title: "用户", dataIndex: "user_id", width: 150 },
            { title: "状态", width: 120, render: (_, row) => <StatusTag value={row.status} /> },
            { title: "模式", dataIndex: "billing_mode", width: 110 },
            { title: "模型", dataIndex: "model_alias", width: 180 },
            { title: "预留点", width: 100, render: (_, row) => coins(row.reserved_coins) },
            { title: "扣点", width: 100, render: (_, row) => coins(row.charged_coins) },
            { title: "扣权益", width: 100, render: (_, row) => coins(row.charged_units) },
            { title: "Tokens", width: 120, render: (_, row) => coins(row.actual_total_tokens) },
            { title: "图片", width: 90, render: (_, row) => coins(row.actual_images) },
            { title: "视频秒", width: 90, render: (_, row) => coins(row.actual_video_seconds) },
            { title: "错误", dataIndex: "error_message", width: 260 }
          ]}
          pagination={{ pageSize: 20 }}
        />
      </Card>
    </div>
  );
}
