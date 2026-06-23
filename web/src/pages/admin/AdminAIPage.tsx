import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, Card, Form, Input, Select, Space, Table } from "antd";
import { RefreshCw, Save } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { api } from "../../api/client";
import type { AIModelItem, AIProviderItem, AppModelRouteItem } from "../../api/types";
import { StatusTag } from "../../components/format";
import { AdminSelect } from "./AdminSelectors";

export function AdminAIPage() {
  const queryClient = useQueryClient();
  const [routeAppCode, setRouteAppCode] = useState("");
  const apps = useQuery({ queryKey: ["admin-apps"], queryFn: api.adminApps });
  const providers = useQuery({ queryKey: ["ai-providers"], queryFn: api.aiProviders });
  const models = useQuery({ queryKey: ["ai-models"], queryFn: api.aiModels });
  const appItems = useMemo(() => apps.data?.items || [], [apps.data?.items]);
  const providerItems = useMemo(() => providers.data?.items || [], [providers.data?.items]);
  const modelItems = useMemo(() => models.data?.items || [], [models.data?.items]);
  const appOptions = useMemo(() => appItems.map((item) => ({ value: item.app_code, label: item.name })), [appItems]);
  const providerOptions = useMemo(() => providerItems.map((item) => ({ value: item.provider_code, label: item.name })), [providerItems]);
  const modelOptions = useMemo(() => modelItems.map((item) => ({ value: item.model_alias, label: item.provider_model })), [modelItems]);
  const routes = useQuery({ queryKey: ["app-model-routes", routeAppCode], queryFn: () => api.appModelRoutes(routeAppCode), enabled: !!routeAppCode });
  const saveProvider = useMutation({
    mutationFn: api.saveAIProvider,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["ai-providers"] })
  });
  const saveModel = useMutation({
    mutationFn: api.saveAIModel,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["ai-models"] })
  });
  const saveRoute = useMutation({
    mutationFn: (values: { model_alias: string; status: string }) => api.saveAppModelRoute(routeAppCode, values),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["app-model-routes", routeAppCode] })
  });

  useEffect(() => {
    if (!routeAppCode && appItems.length > 0) {
      setRouteAppCode(appItems[0].app_code);
    }
  }, [appItems, routeAppCode]);

  return (
    <div className="page-stack">
      <div className="page-title-row">
        <h1 className="page-title">AI 配置</h1>
        <Button icon={<RefreshCw size={16} />} onClick={() => queryClient.invalidateQueries()}>刷新</Button>
      </div>
      <div className="three-grid">
        <Card title="供应商">
          <Form layout="vertical" initialValues={{ provider_code: "openai", name: "OpenAI via LiteLLM", status: "active", base_url: "https://api.openai.com" }} onFinish={(values) => saveProvider.mutate(values)}>
            <Form.Item label="Provider Code" name="provider_code" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item label="名称" name="name" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item label="状态" name="status"><Select options={[{ value: "active" }, { value: "disabled" }]} /></Form.Item>
            <Form.Item label="Base URL" name="base_url"><Input /></Form.Item>
            <Button type="primary" htmlType="submit" icon={<Save size={16} />} loading={saveProvider.isPending}>保存</Button>
          </Form>
        </Card>
        <Card title="模型">
          <Form layout="vertical" initialValues={{ model_alias: "openai-chat-default", provider_model: "gpt-4o-mini", capability: "chat", status: "active" }} onFinish={(values) => saveModel.mutate(values)}>
            <Form.Item label="Provider Code" name="provider_code" rules={[{ required: true, message: "请选择供应商" }]}>
              <AdminSelect options={providerOptions} placeholder="选择供应商" />
            </Form.Item>
            <Form.Item label="模型别名" name="model_alias" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item label="供应商模型" name="provider_model" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item label="能力" name="capability"><Select options={[{ value: "chat" }, { value: "image" }, { value: "audio" }, { value: "video" }, { value: "embedding" }]} /></Form.Item>
            <Form.Item label="状态" name="status"><Select options={[{ value: "active" }, { value: "disabled" }]} /></Form.Item>
            <Button type="primary" htmlType="submit" icon={<Save size={16} />} loading={saveModel.isPending}>保存</Button>
          </Form>
        </Card>
        <Card title="应用路由">
          <Form layout="vertical" initialValues={{ status: "active" }} onFinish={(values) => saveRoute.mutate(values)}>
            <Form.Item label="App Code" required>
              <AdminSelect value={routeAppCode} onChange={(value) => setRouteAppCode(value || "")} options={appOptions} placeholder="选择应用" />
            </Form.Item>
            <Form.Item label="模型别名" name="model_alias" rules={[{ required: true, message: "请选择模型" }]}>
              <AdminSelect options={modelOptions} placeholder="选择模型" />
            </Form.Item>
            <Form.Item label="状态" name="status"><Select options={[{ value: "active" }, { value: "disabled" }]} /></Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" icon={<Save size={16} />} loading={saveRoute.isPending}>保存</Button>
              <Button onClick={() => routes.refetch()}>查看</Button>
            </Space>
          </Form>
        </Card>
      </div>
      <div className="split-grid">
        <Card title="供应商列表" className="table-card">
          <Table<AIProviderItem>
            rowKey="id"
            size="small"
            loading={providers.isPending}
            dataSource={providers.data?.items || []}
            pagination={false}
            columns={[
              { title: "Code", dataIndex: "provider_code" },
              { title: "名称", dataIndex: "name" },
              { title: "状态", render: (_, row) => <StatusTag value={row.status} /> },
              { title: "Base URL", dataIndex: "base_url" }
            ]}
          />
        </Card>
        <Card title="模型列表" className="table-card">
          <Table<AIModelItem>
            rowKey="id"
            size="small"
            loading={models.isPending}
            dataSource={models.data?.items || []}
            pagination={false}
            columns={[
              { title: "别名", dataIndex: "model_alias" },
              { title: "供应商", dataIndex: "provider_code" },
              { title: "真实模型", dataIndex: "provider_model" },
              { title: "能力", dataIndex: "capability" },
              { title: "状态", render: (_, row) => <StatusTag value={row.status} /> }
            ]}
          />
        </Card>
      </div>
      <Card title={`应用模型路由：${routeAppCode}`} className="table-card">
        <Table<AppModelRouteItem>
          rowKey="id"
          size="small"
          loading={routes.isPending}
          dataSource={routes.data?.items || []}
          pagination={false}
          columns={[
            { title: "应用", dataIndex: "app_code" },
            { title: "模型", dataIndex: "model_alias" },
            { title: "供应商", dataIndex: "provider_code" },
            { title: "真实模型", dataIndex: "provider_model" },
            { title: "能力", dataIndex: "capability" },
            { title: "状态", render: (_, row) => <StatusTag value={row.status} /> }
          ]}
        />
      </Card>
    </div>
  );
}
