import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { App, Button, Card, Descriptions, Form, Input, InputNumber, Space, Table, Tabs } from "antd";
import { RefreshCw, Search } from "lucide-react";
import { useState } from "react";
import { api } from "../../api/client";
import type {
  EntitlementItem,
  EntitlementLedgerItem,
  RechargeOrderItem,
  SubscriptionItem,
  UsageRequestItem,
  WalletLedgerItem
} from "../../api/types";
import { cents, coins, points, StatusTag, time } from "../../components/format";
import { AdminUserSearchSelect } from "./AdminSelectors";

export function AdminUsersPage() {
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [userID, setUserID] = useState("");
  const enabled = !!userID.trim();
  const tableLoading = (isFetching: boolean) => enabled && isFetching;
  const asset = useQuery({ queryKey: ["admin-user-asset", userID], queryFn: () => api.adminUserAsset(userID), enabled });
  const ledger = useQuery({
    queryKey: ["admin-user-wallet-ledger", userID],
    queryFn: () => api.adminUserWalletLedger(userID, 50),
    enabled
  });
  const subscriptions = useQuery({
    queryKey: ["admin-user-subscriptions", userID],
    queryFn: () => api.adminUserSubscriptions(userID),
    enabled
  });
  const entitlements = useQuery({
    queryKey: ["admin-user-entitlements", userID],
    queryFn: () => api.adminUserEntitlements(userID),
    enabled
  });
  const entitlementLedger = useQuery({
    queryKey: ["admin-user-entitlement-ledger", userID],
    queryFn: () => api.adminUserEntitlementLedger(userID, 50),
    enabled
  });
  const usageRequests = useQuery({
    queryKey: ["admin-user-usage-requests", userID],
    queryFn: () => api.adminUserUsageRequests(userID, 50),
    enabled
  });
  const rechargeOrders = useQuery({
    queryKey: ["admin-user-recharge-orders", userID],
    queryFn: () => api.adminUserRechargeOrders(userID, 50),
    enabled
  });
  const adjust = useMutation({
    mutationFn: (values: { amount_coins: number; remark: string }) => api.adminAdjustCoins(userID, values),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["admin-user-asset", userID] });
      await queryClient.invalidateQueries({ queryKey: ["admin-user-wallet-ledger", userID] });
      message.success("调账完成");
    }
  });
  const refreshAll = () => {
    queryClient.invalidateQueries({ queryKey: ["admin-user-asset", userID] });
    queryClient.invalidateQueries({ queryKey: ["admin-user-wallet-ledger", userID] });
    queryClient.invalidateQueries({ queryKey: ["admin-user-subscriptions", userID] });
    queryClient.invalidateQueries({ queryKey: ["admin-user-entitlements", userID] });
    queryClient.invalidateQueries({ queryKey: ["admin-user-entitlement-ledger", userID] });
    queryClient.invalidateQueries({ queryKey: ["admin-user-usage-requests", userID] });
    queryClient.invalidateQueries({ queryKey: ["admin-user-recharge-orders", userID] });
  };

  return (
    <div className="page-stack">
      <div className="page-title-row">
        <h1 className="page-title">用户资产</h1>
        <Space align="start">
          <AdminUserSearchSelect value={userID} onChange={(value) => setUserID(value || "")} width={360} />
          <Button icon={<Search size={16} />} onClick={refreshAll} disabled={!enabled} />
          <Button icon={<RefreshCw size={16} />} onClick={refreshAll} disabled={!enabled} />
        </Space>
      </div>
      <div className="split-grid">
        <Card title="资产">
          <Descriptions bordered size="small" column={1}>
            <Descriptions.Item label="用户">{asset.data?.user.casdoor_user_id || "-"}</Descriptions.Item>
            <Descriptions.Item label="名称">{asset.data?.user.display_name || asset.data?.user.name || "-"}</Descriptions.Item>
            <Descriptions.Item label="邮箱">{asset.data?.user.email || "-"}</Descriptions.Item>
            <Descriptions.Item label="管理员"><StatusTag value={asset.data?.user.is_admin ? "active" : "disabled"} /></Descriptions.Item>
            <Descriptions.Item label="总余额">{points(asset.data?.wallet.balance_coins)}</Descriptions.Item>
            <Descriptions.Item label="可用">{points(asset.data?.wallet.available_coins)}</Descriptions.Item>
            <Descriptions.Item label="冻结">{points(asset.data?.wallet.reserved_coins)}</Descriptions.Item>
          </Descriptions>
        </Card>
        <Card title="手工调账">
          <Form layout="vertical" initialValues={{ amount_coins: 50, remark: "manual adjustment" }} onFinish={(values) => adjust.mutate(values)}>
            <Form.Item label="点数增减" name="amount_coins" rules={[{ required: true }]}>
              <InputNumber style={{ width: "100%" }} />
            </Form.Item>
            <Form.Item label="备注" name="remark" rules={[{ required: true }]}>
              <Input />
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={adjust.isPending}>提交调账</Button>
          </Form>
        </Card>
      </div>
      <Card className="table-card">
        <Tabs
          items={[
            {
              key: "wallet",
              label: "钱包流水",
              children: (
                <Table<WalletLedgerItem>
                  rowKey="id"
                  size="small"
                  loading={tableLoading(ledger.isFetching)}
                  dataSource={ledger.data?.items || []}
                  columns={[
                    { title: "时间", render: (_, row) => time(row.created_at) },
                    { title: "方向", dataIndex: "direction" },
                    { title: "原因", dataIndex: "reason" },
                    { title: "数量", render: (_, row) => coins(row.amount_coins) },
                    { title: "余额", render: (_, row) => coins(row.balance_after) },
                    { title: "备注", dataIndex: "remark" }
                  ]}
                  pagination={{ pageSize: 12 }}
                />
              )
            },
            {
              key: "subscriptions",
              label: "已订套餐",
              children: (
                <Table<SubscriptionItem>
                  rowKey="id"
                  size="small"
                  loading={tableLoading(subscriptions.isFetching)}
                  dataSource={subscriptions.data?.items || []}
                  columns={[
                    { title: "应用", dataIndex: "app_code" },
                    { title: "套餐", dataIndex: "plan_code" },
                    { title: "名称", dataIndex: "plan_name" },
                    { title: "状态", render: (_, row) => <StatusTag value={row.status} /> },
                    { title: "开始", render: (_, row) => time(row.starts_at) },
                    { title: "结束", render: (_, row) => time(row.ends_at) }
                  ]}
                  pagination={false}
                />
              )
            },
            {
              key: "entitlements",
              label: "权益余额",
              children: (
                <Table<EntitlementItem>
                  rowKey="id"
                  size="small"
                  loading={tableLoading(entitlements.isFetching)}
                  dataSource={entitlements.data?.items || []}
                  columns={[
                    { title: "应用", dataIndex: "app_code" },
                    { title: "权益", dataIndex: "entitlement_code" },
                    { title: "单位", dataIndex: "unit" },
                    { title: "月份", dataIndex: "period_month" },
                    { title: "额度", render: (_, row) => coins(row.quota) },
                    { title: "已用", render: (_, row) => coins(row.used) },
                    { title: "冻结", render: (_, row) => coins(row.reserved) },
                    { title: "可用", render: (_, row) => coins(row.available) }
                  ]}
                  pagination={false}
                />
              )
            },
            {
              key: "entitlement-ledger",
              label: "权益流水",
              children: (
                <Table<EntitlementLedgerItem>
                  rowKey="id"
                  size="small"
                  loading={tableLoading(entitlementLedger.isFetching)}
                  dataSource={entitlementLedger.data?.items || []}
                  columns={[
                    { title: "时间", render: (_, row) => time(row.created_at) },
                    { title: "应用", dataIndex: "app_code" },
                    { title: "权益", dataIndex: "entitlement_code" },
                    { title: "方向", dataIndex: "direction" },
                    { title: "数量", render: (_, row) => coins(row.amount) },
                    { title: "已用后", render: (_, row) => coins(row.used_after) },
                    { title: "冻结后", render: (_, row) => coins(row.reserved_after) }
                  ]}
                  pagination={{ pageSize: 12 }}
                />
              )
            },
            {
              key: "usage",
              label: "AI 调用",
              children: (
                <Table<UsageRequestItem>
                  rowKey="id"
                  size="small"
                  loading={tableLoading(usageRequests.isFetching)}
                  dataSource={usageRequests.data?.items || []}
                  columns={[
                    { title: "时间", render: (_, row) => time(row.created_at) },
                    { title: "应用", dataIndex: "app_code" },
                    { title: "模型", dataIndex: "model_alias" },
                    { title: "状态", render: (_, row) => <StatusTag value={row.status} /> },
                    { title: "Token", render: (_, row) => coins(row.actual_total_tokens || row.estimated_total_tokens) },
                    { title: "图片", render: (_, row) => coins(row.actual_images || row.estimated_images) },
                    { title: "视频秒", render: (_, row) => coins(row.actual_video_seconds || row.estimated_video_seconds) },
                    { title: "扣点", render: (_, row) => coins(row.charged_coins) },
                    { title: "权益", render: (_, row) => coins(row.charged_units) },
                    { title: "错误", dataIndex: "error_code" }
                  ]}
                  pagination={{ pageSize: 12 }}
                />
              )
            },
            {
              key: "recharge",
              label: "充值订单",
              children: (
                <Table<RechargeOrderItem>
                  rowKey="id"
                  size="small"
                  loading={tableLoading(rechargeOrders.isFetching)}
                  dataSource={rechargeOrders.data?.items || []}
                  columns={[
                    { title: "时间", render: (_, row) => time(row.created_at) },
                    { title: "订单号", dataIndex: "order_no" },
                    { title: "金额", render: (_, row) => cents(row.amount_cents) },
                    { title: "点数", render: (_, row) => coins(row.coins) },
                    { title: "状态", render: (_, row) => <StatusTag value={row.status} /> },
                    { title: "支付时间", render: (_, row) => time(row.paid_at) }
                  ]}
                  pagination={{ pageSize: 12 }}
                />
              )
            }
          ]}
        />
      </Card>
    </div>
  );
}
