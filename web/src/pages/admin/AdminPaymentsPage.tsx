import { useQuery } from "@tanstack/react-query";
import { Button, Card, Form, Input, Select, Table } from "antd";
import { Search } from "lucide-react";
import { useMemo, useState } from "react";
import { api } from "../../api/client";
import type { PaymentEventItem, RechargeOrderItem } from "../../api/types";
import { cents, coins, StatusTag, time } from "../../components/format";
import { AdminUserSearchSelect } from "./AdminSelectors";

export function AdminPaymentsPage() {
  const [orderFilter, setOrderFilter] = useState({ user_id: "", status: "", limit: 100 });
  const [eventFilter, setEventFilter] = useState({ order_no: "", limit: 100 });
  const orderParams = useMemo(() => {
    const next = new URLSearchParams();
    if (orderFilter.user_id) next.set("user_id", orderFilter.user_id);
    if (orderFilter.status) next.set("status", orderFilter.status);
    next.set("limit", String(orderFilter.limit));
    return next;
  }, [orderFilter]);
  const eventParams = useMemo(() => {
    const next = new URLSearchParams();
    if (eventFilter.order_no) next.set("order_no", eventFilter.order_no);
    next.set("limit", String(eventFilter.limit));
    return next;
  }, [eventFilter]);
  const orders = useQuery({ queryKey: ["admin-recharge-orders", orderParams.toString()], queryFn: () => api.adminRechargeOrders(orderParams) });
  const events = useQuery({ queryKey: ["admin-payment-events", eventParams.toString()], queryFn: () => api.adminPaymentEvents(eventParams) });

  return (
    <div className="page-stack">
      <div className="page-title-row">
        <h1 className="page-title">支付记录</h1>
      </div>
      <Card title="充值订单">
        <Form layout="inline" initialValues={orderFilter} onFinish={(values) => setOrderFilter(values)}>
          <Form.Item label="用户" name="user_id"><AdminUserSearchSelect width={320} /></Form.Item>
          <Form.Item label="状态" name="status"><Select allowClear style={{ width: 130 }} options={[{ value: "pending" }, { value: "paid" }, { value: "closed" }, { value: "failed" }]} /></Form.Item>
          <Form.Item label="数量" name="limit"><Input allowClear /></Form.Item>
          <Button type="primary" htmlType="submit" icon={<Search size={16} />}>查询</Button>
        </Form>
        <Table<RechargeOrderItem>
          rowKey="id"
          size="small"
          loading={orders.isPending}
          dataSource={orders.data?.items || []}
          style={{ marginTop: 12 }}
          columns={[
            { title: "时间", render: (_, row) => time(row.created_at) },
            { title: "用户", dataIndex: "user_id" },
            { title: "订单号", dataIndex: "order_no" },
            { title: "金额", render: (_, row) => cents(row.amount_cents) },
            { title: "点数", render: (_, row) => coins(row.coins) },
            { title: "状态", render: (_, row) => <StatusTag value={row.status} /> },
            { title: "交易号", dataIndex: "provider_trade_no" }
          ]}
          pagination={{ pageSize: 12 }}
        />
      </Card>
      <Card title="支付事件">
        <Form layout="inline" initialValues={eventFilter} onFinish={(values) => setEventFilter(values)}>
          <Form.Item label="订单号" name="order_no"><Input allowClear style={{ width: 320 }} /></Form.Item>
          <Form.Item label="数量" name="limit"><Input allowClear /></Form.Item>
          <Button type="primary" htmlType="submit" icon={<Search size={16} />}>查询</Button>
        </Form>
        <Table<PaymentEventItem>
          rowKey="id"
          size="small"
          loading={events.isPending}
          dataSource={events.data?.items || []}
          style={{ marginTop: 12 }}
          columns={[
            { title: "时间", render: (_, row) => time(row.created_at) },
            { title: "事件", dataIndex: "event_id" },
            { title: "类型", dataIndex: "event_type" },
            { title: "订单号", dataIndex: "order_no" },
            { title: "交易号", dataIndex: "provider_trade_no" },
            { title: "处理时间", render: (_, row) => time(row.processed_at) }
          ]}
          pagination={{ pageSize: 12 }}
        />
      </Card>
    </div>
  );
}
