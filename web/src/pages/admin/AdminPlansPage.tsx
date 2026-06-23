import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { App, Button, Card, Form, Input, InputNumber, Select, Space, Table } from "antd";
import { Boxes, RefreshCw } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { api } from "../../api/client";
import type { EntitlementMaintenanceResult, PlanItem } from "../../api/types";
import { cents, coins, StatusTag } from "../../components/format";
import { AdminSelect, AdminUserSearchSelect } from "./AdminSelectors";

export function AdminPlansPage() {
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [grantForm] = Form.useForm();
  const [appCode, setAppCode] = useState("");
  const [planCode, setPlanCode] = useState("");
  const apps = useQuery({ queryKey: ["admin-apps"], queryFn: api.adminApps });
  const appItems = useMemo(() => apps.data?.items || [], [apps.data?.items]);
  const appOptions = useMemo(() => appItems.map((item) => ({ value: item.app_code, label: item.name })), [appItems]);
  const plans = useQuery({ queryKey: ["admin-plans", appCode], queryFn: () => api.adminPlans(appCode), enabled: !!appCode });
  const planItems = useMemo(() => plans.data?.items || [], [plans.data?.items]);
  const planOptions = useMemo(() => planItems.map((item) => ({ value: item.plan_code, label: item.name })), [planItems]);
  const savePlan = useMutation({
    mutationFn: (values: { plan_code: string; name: string; period: string; price_cents: number; status: string }) =>
      api.adminSavePlan(appCode, values),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["admin-plans", appCode] })
  });
  const saveEntitlement = useMutation({
    mutationFn: (values: { entitlement_code: string; name: string; unit: string; monthly_quota: number }) =>
      api.adminSavePlanEntitlement(appCode, planCode, values),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["admin-plans", appCode] })
  });
  const grant = useMutation({
    mutationFn: (values: { user_id: string; app_code: string; plan_code: string; months: number }) =>
      api.adminGrantSubscription(values.user_id, values),
    onSuccess: () => message.success("套餐已开通")
  });
  const maintenance = useMutation({
    mutationFn: api.adminRunEntitlementMaintenance,
    onSuccess: (data: EntitlementMaintenanceResult) => message.success(`维护完成：新增 ${data.created_balances}`)
  });

  useEffect(() => {
    if (!appCode && appItems.length > 0) {
      setAppCode(appItems[0].app_code);
    }
  }, [appCode, appItems]);

  useEffect(() => {
    grantForm.setFieldValue("app_code", appCode || undefined);
  }, [appCode, grantForm]);

  useEffect(() => {
    if (plans.isSuccess && planItems.length === 0 && planCode) {
      setPlanCode("");
      grantForm.setFieldValue("plan_code", undefined);
      return;
    }
    if (!planCode && planItems.length > 0) {
      setPlanCode(planItems[0].plan_code);
      grantForm.setFieldValue("plan_code", planItems[0].plan_code);
      return;
    }
    if (planCode && planItems.length > 0 && !planItems.some((item) => item.plan_code === planCode)) {
      setPlanCode(planItems[0].plan_code);
      grantForm.setFieldValue("plan_code", planItems[0].plan_code);
    }
  }, [grantForm, planCode, planItems, plans.isSuccess]);

  return (
    <div className="page-stack">
      <div className="page-title-row">
        <h1 className="page-title">套餐权益</h1>
        <Space>
          <AdminSelect
            value={appCode}
            onChange={(value) => {
              setAppCode(value || "");
              setPlanCode("");
              grantForm.setFieldsValue({ app_code: value, plan_code: undefined });
            }}
            options={appOptions}
            placeholder="选择应用"
            width={220}
          />
          <Button icon={<RefreshCw size={16} />} onClick={() => plans.refetch()} />
        </Space>
      </div>
      <div className="three-grid">
        <Card title="套餐">
          <Form layout="vertical" initialValues={{ plan_code: "starter", name: "Starter Monthly", period: "monthly", price_cents: 990, status: "active" }} onFinish={(values) => savePlan.mutate(values)}>
            <Form.Item label="套餐 Code" name="plan_code" rules={[{ required: true }]}><Input onChange={(event) => setPlanCode(event.target.value)} /></Form.Item>
            <Form.Item label="名称" name="name" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item label="周期" name="period"><Select options={[{ value: "monthly" }, { value: "yearly" }]} /></Form.Item>
            <Form.Item label="价格/分" name="price_cents"><InputNumber min={0} style={{ width: "100%" }} /></Form.Item>
            <Form.Item label="状态" name="status"><Select options={[{ value: "active" }, { value: "disabled" }]} /></Form.Item>
            <Button type="primary" htmlType="submit" loading={savePlan.isPending}>保存套餐</Button>
          </Form>
        </Card>
        <Card title="权益">
          <Form layout="vertical" initialValues={{ entitlement_code: "monthly_tokens", name: "Monthly Tokens", unit: "tokens", monthly_quota: 200000 }} onFinish={(values) => saveEntitlement.mutate(values)}>
            <Form.Item label="套餐 Code" required>
              <AdminSelect value={planCode} onChange={(value) => setPlanCode(value || "")} options={planOptions} placeholder="选择套餐" />
            </Form.Item>
            <Form.Item label="权益 Code" name="entitlement_code" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item label="名称" name="name" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item label="单位" name="unit"><Select options={[{ value: "tokens" }, { value: "images" }, { value: "video_seconds" }, { value: "business_units" }]} /></Form.Item>
            <Form.Item label="每月额度" name="monthly_quota"><InputNumber min={0} style={{ width: "100%" }} /></Form.Item>
            <Button type="primary" htmlType="submit" loading={saveEntitlement.isPending}>保存权益</Button>
          </Form>
        </Card>
        <Card title="开通与维护">
          <Form
            form={grantForm}
            layout="vertical"
            initialValues={{ months: 1 }}
            onValuesChange={(changed) => {
              if (Object.prototype.hasOwnProperty.call(changed, "app_code")) {
                setAppCode(changed.app_code || "");
                setPlanCode("");
                grantForm.setFieldValue("plan_code", undefined);
              }
              if (Object.prototype.hasOwnProperty.call(changed, "plan_code")) {
                setPlanCode(changed.plan_code || "");
              }
            }}
            onFinish={(values) => grant.mutate(values)}
          >
            <Form.Item label="用户" name="user_id" rules={[{ required: true, message: "请选择用户" }]}><AdminUserSearchSelect /></Form.Item>
            <Form.Item label="App Code" name="app_code" rules={[{ required: true, message: "请选择应用" }]}>
              <AdminSelect options={appOptions} placeholder="选择应用" />
            </Form.Item>
            <Form.Item label="套餐 Code" name="plan_code" rules={[{ required: true, message: "请选择套餐" }]}>
              <AdminSelect options={planOptions} placeholder="选择套餐" />
            </Form.Item>
            <Form.Item label="月数" name="months"><InputNumber min={1} style={{ width: "100%" }} /></Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" icon={<Boxes size={16} />} loading={grant.isPending}>开通</Button>
              <Button onClick={() => maintenance.mutate()} loading={maintenance.isPending}>执行维护</Button>
            </Space>
          </Form>
        </Card>
      </div>
      <Card title="套餐列表" className="table-card">
        <Table<PlanItem>
          rowKey="id"
          size="small"
          loading={plans.isPending}
          dataSource={planItems}
          onRow={(row) => ({
            onClick: () => {
              setPlanCode(row.plan_code);
              grantForm.setFieldValue("plan_code", row.plan_code);
            }
          })}
          columns={[
            { title: "套餐", dataIndex: "plan_code" },
            { title: "名称", dataIndex: "name" },
            { title: "周期", dataIndex: "period" },
            { title: "价格", render: (_, row) => cents(row.price_cents) },
            { title: "状态", render: (_, row) => <StatusTag value={row.status} /> },
            { title: "权益", render: (_, row) => (row.entitlements || []).map((item) => `${item.entitlement_code}: ${coins(item.monthly_quota)} ${item.unit}`).join("; ") }
          ]}
          pagination={false}
        />
      </Card>
    </div>
  );
}
