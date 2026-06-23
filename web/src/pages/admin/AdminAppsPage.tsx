import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { App, Button, Card, Form, Input, InputNumber, Popconfirm, Select, Space, Table, Tabs } from "antd";
import { KeyRound, Pencil, Save, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { api } from "../../api/client";
import type { AdminAppItem, TokenPricingItem, UnitPricingItem } from "../../api/types";
import { coins, StatusTag, time } from "../../components/format";
import { AdminSelect } from "./AdminSelectors";

export function AdminAppsPage() {
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [tokenForm] = Form.useForm();
  const [unitForm] = Form.useForm();
  const [appCode, setAppCode] = useState("");
  const apps = useQuery({ queryKey: ["admin-apps"], queryFn: api.adminApps });
  const models = useQuery({ queryKey: ["ai-models"], queryFn: api.aiModels });
  const appItems = useMemo(() => apps.data?.items || [], [apps.data?.items]);
  const appOptions = useMemo(() => appItems.map((item) => ({ value: item.app_code, label: item.name })), [appItems]);
  const modelOptions = useMemo(
    () => (models.data?.items || []).map((item) => ({ value: item.model_alias, label: item.provider_model })),
    [models.data?.items]
  );
  const existingAppCode = appItems.some((item) => item.app_code === appCode);
  const pricing = useQuery({
    queryKey: ["admin-pricing", appCode],
    queryFn: () => api.adminPricing(appCode),
    enabled: !!appCode && existingAppCode
  });
  const saveApp = useMutation({
    mutationFn: api.adminSaveApp,
    onSuccess: async (data) => {
      setAppCode(data.app_code);
      await queryClient.invalidateQueries({ queryKey: ["admin-apps"] });
      await queryClient.invalidateQueries({ queryKey: ["admin-pricing", data.app_code] });
    }
  });
  const createKey = useMutation({
    mutationFn: api.adminCreateAPIKey,
    onSuccess: (data) => message.success(`API Key: ${data.key}`)
  });
  const savePricing = useMutation({
    mutationFn: (values: { model_alias: string; token_amount: number; coin_amount: number }) => api.adminSavePricing(appCode, values),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["admin-pricing", appCode] })
  });
  const deleteTokenPricing = useMutation({
    mutationFn: api.adminDeleteTokenPricing,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["admin-pricing", appCode] })
  });
  const saveUnitPricing = useMutation({
    mutationFn: (values: { model_alias: string; unit: string; unit_amount: number; coin_amount: number }) =>
      api.adminSaveUnitPricing(appCode, values),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["admin-pricing", appCode] })
  });
  const deleteUnitPricing = useMutation({
    mutationFn: api.adminDeleteUnitPricing,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["admin-pricing", appCode] })
  });

  useEffect(() => {
    if (!appCode && appItems.length > 0) {
      setAppCode(appItems[0].app_code);
    }
  }, [appCode, appItems]);

  return (
    <div className="page-stack">
      <div className="page-title-row">
        <h1 className="page-title">应用与定价</h1>
      </div>
      <div className="split-grid">
        <Card title="应用">
          <Form layout="vertical" initialValues={{ app_code: "demo-app", name: "Demo AI App", billing_mode: "hybrid", description: "" }} onFinish={(values) => saveApp.mutate(values)}>
            <Form.Item label="App Code" name="app_code" rules={[{ required: true }]}>
              <Input onChange={(event) => setAppCode(event.target.value)} />
            </Form.Item>
            <Form.Item label="名称" name="name" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item label="扣费方式" name="billing_mode"><Select options={[{ value: "hybrid" }, { value: "coins" }, { value: "entitlement" }]} /></Form.Item>
            <Form.Item label="描述" name="description"><Input.TextArea rows={3} /></Form.Item>
            <Button type="primary" htmlType="submit" loading={saveApp.isPending} icon={<Save size={16} />}>保存应用</Button>
          </Form>
        </Card>
        <Card title="API Key">
          <Form layout="vertical">
            <Form.Item label="App Code" required>
              <AdminSelect value={appCode} onChange={(value) => setAppCode(value || "")} options={appOptions} placeholder="选择应用" />
            </Form.Item>
            <Button icon={<KeyRound size={16} />} onClick={() => createKey.mutate(appCode)} loading={createKey.isPending} disabled={!existingAppCode}>生成 API Key</Button>
          </Form>
        </Card>
      </div>
      <Card title="应用列表" className="table-card">
        <Table<AdminAppItem>
          rowKey="id"
          size="small"
          loading={apps.isPending}
          dataSource={appItems}
          onRow={(row) => ({ onClick: () => setAppCode(row.app_code) })}
          columns={[
            { title: "App Code", dataIndex: "app_code" },
            { title: "名称", dataIndex: "name" },
            { title: "状态", render: (_, row) => <StatusTag value={row.status} /> },
            { title: "扣费", render: (_, row) => <StatusTag value={row.billing_mode} /> },
            { title: "描述", dataIndex: "description" }
          ]}
          pagination={false}
        />
      </Card>
      <Card title={appCode && existingAppCode ? `定价：${appCode}` : "定价"}>
        <Tabs
          items={[
            {
              key: "token",
              label: "Token 定价",
              children: (
                <>
                  <Form
                    form={tokenForm}
                    layout="inline"
                    onFinish={(values) => savePricing.mutate(values)}
                    initialValues={{ token_amount: 1000, coin_amount: 1 }}
                    disabled={!existingAppCode}
                  >
                    <Form.Item name="model_alias" rules={[{ required: true, message: "请选择模型" }]}>
                      <AdminSelect options={modelOptions} placeholder="模型" width={180} />
                    </Form.Item>
                    <Form.Item name="token_amount"><InputNumber min={1} placeholder="Token 数" /></Form.Item>
                    <Form.Item name="coin_amount"><InputNumber min={1} placeholder="点数" /></Form.Item>
                    <Button htmlType="submit" type="primary">保存</Button>
                  </Form>
                  <Table<TokenPricingItem>
                    style={{ marginTop: 12 }}
                    rowKey="id"
                    size="small"
                    loading={pricing.isPending && existingAppCode}
                    dataSource={pricing.data?.token_pricing || []}
                    pagination={false}
                    columns={[
                      { title: "模型", dataIndex: "model_alias" },
                      { title: "Token", render: (_, row) => coins(row.token_amount) },
                      { title: "点数", render: (_, row) => coins(row.coin_amount) },
                      { title: "状态", render: (_, row) => <StatusTag value={row.status} /> },
                      { title: "生效", render: (_, row) => time(row.effective_from) },
                      {
                        title: "操作",
                        render: (_, row) => (
                          <Space size={6}>
                            <Button
                              size="small"
                              icon={<Pencil size={14} />}
                              onClick={() =>
                                tokenForm.setFieldsValue({
                                  model_alias: row.model_alias,
                                  token_amount: row.token_amount,
                                  coin_amount: row.coin_amount
                                })
                              }
                            />
                            <Popconfirm title="删除这条 Token 定价？" onConfirm={() => deleteTokenPricing.mutate(row.id)}>
                              <Button size="small" danger icon={<Trash2 size={14} />} loading={deleteTokenPricing.isPending} />
                            </Popconfirm>
                          </Space>
                        )
                      }
                    ]}
                  />
                </>
              )
            },
            {
              key: "unit",
              label: "单位定价",
              children: (
                <>
                  <Form
                    form={unitForm}
                    layout="inline"
                    onFinish={(values) => saveUnitPricing.mutate(values)}
                    initialValues={{ unit: "images", unit_amount: 1, coin_amount: 20 }}
                    disabled={!existingAppCode}
                  >
                    <Form.Item name="model_alias" rules={[{ required: true, message: "请选择模型" }]}>
                      <AdminSelect options={modelOptions} placeholder="模型" width={180} />
                    </Form.Item>
                    <Form.Item name="unit"><Select style={{ width: 150 }} options={[{ value: "images" }, { value: "video_seconds" }, { value: "business_units" }]} /></Form.Item>
                    <Form.Item name="unit_amount"><InputNumber min={1} placeholder="单位数量" /></Form.Item>
                    <Form.Item name="coin_amount"><InputNumber min={1} placeholder="点数" /></Form.Item>
                    <Button htmlType="submit" type="primary">保存</Button>
                  </Form>
                  <Table<UnitPricingItem>
                    style={{ marginTop: 12 }}
                    rowKey="id"
                    size="small"
                    loading={pricing.isPending && existingAppCode}
                    dataSource={pricing.data?.unit_pricing || []}
                    pagination={false}
                    columns={[
                      { title: "模型", dataIndex: "model_alias" },
                      { title: "单位", dataIndex: "unit" },
                      { title: "单位数量", render: (_, row) => coins(row.unit_amount) },
                      { title: "点数", render: (_, row) => coins(row.coin_amount) },
                      { title: "状态", render: (_, row) => <StatusTag value={row.status} /> },
                      {
                        title: "操作",
                        render: (_, row) => (
                          <Space size={6}>
                            <Button
                              size="small"
                              icon={<Pencil size={14} />}
                              onClick={() =>
                                unitForm.setFieldsValue({
                                  model_alias: row.model_alias,
                                  unit: row.unit,
                                  unit_amount: row.unit_amount,
                                  coin_amount: row.coin_amount
                                })
                              }
                            />
                            <Popconfirm title="删除这条单位定价？" onConfirm={() => deleteUnitPricing.mutate(row.id)}>
                              <Button size="small" danger icon={<Trash2 size={14} />} loading={deleteUnitPricing.isPending} />
                            </Popconfirm>
                          </Space>
                        )
                      }
                    ]}
                  />
                </>
              )
            }
          ]}
        />
      </Card>
    </div>
  );
}
