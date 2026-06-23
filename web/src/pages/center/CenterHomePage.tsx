import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { App, Button, Card, Empty, Form, Input, Modal, Progress, Statistic, Typography, Upload } from "antd";
import { Camera, Pencil, Save } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { api } from "../../api/client";
import type { EntitlementItem, EntitlementLedgerItem, SubscriptionItem, UsageRequestItem, WalletLedgerItem } from "../../api/types";
import { coins, date, points, time } from "../../components/format";

export function CenterHomePage() {
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<{ display_name: string }>();
  const [avatarVersion, setAvatarVersion] = useState("");
  const [nicknameOpen, setNicknameOpen] = useState(false);
  const me = useQuery({ queryKey: ["me"], queryFn: api.me });
  const wallet = useQuery({ queryKey: ["wallet"], queryFn: api.wallet });
  const ledger = useQuery({ queryKey: ["wallet-ledger"], queryFn: () => api.walletLedger(20) });
  const subscriptions = useQuery({ queryKey: ["subscriptions"], queryFn: api.subscriptions });
  const entitlements = useQuery({ queryKey: ["entitlements"], queryFn: api.entitlements });
  const usage = useQuery({ queryKey: ["usage-requests"], queryFn: () => api.usageRequests(20) });
  const entitlementLedger = useQuery({ queryKey: ["entitlement-ledger"], queryFn: () => api.entitlementLedger(20) });
  const displayName = me.data?.display_name || me.data?.phone || "用户";
  const avatarURL = avatarSrc(me.data?.avatar_url, avatarVersion || me.data?.updated_at);

  useEffect(() => {
    form.setFieldsValue({ display_name: displayName });
  }, [displayName, form]);

  const updateProfile = useMutation({
    mutationFn: (values: { display_name: string }) => api.updateProfile(values),
    onSuccess: (user) => {
      queryClient.setQueryData(["me"], user);
      setNicknameOpen(false);
      message.success("昵称已更新");
    },
    onError: (error) => message.error(error instanceof Error ? error.message : "昵称更新失败"),
    meta: { skipGlobalError: true }
  });

  const uploadAvatar = useMutation({
    mutationFn: (file: File) => api.uploadAvatar(file),
    onSuccess: (user) => {
      const refreshedAt = new Date().toISOString();
      queryClient.setQueryData(["me"], { ...user, updated_at: refreshedAt });
      setAvatarVersion(refreshedAt);
      message.success("头像已更新");
    },
    onError: (error) => message.error(error instanceof Error ? error.message : "头像更新失败"),
    meta: { skipGlobalError: true }
  });

  return (
    <div className="page-stack">
      <div className="page-title-row">
        <h1 className="page-title">用户中心</h1>
      </div>

      <Card className="profile-card">
        <div className="profile-card-inner">
          <div className="profile-avatar-wrap">
            <img className="profile-avatar" src={avatarURL} alt="" />
            <Upload
              accept="image/png,image/jpeg,image/webp,image/gif"
              showUploadList={false}
              beforeUpload={(file) => {
                uploadAvatar.mutate(file);
                return Upload.LIST_IGNORE;
              }}
            >
              <Button icon={<Camera size={16} />} loading={uploadAvatar.isPending}>
                更换头像
              </Button>
            </Upload>
          </div>
          <div className="profile-main">
            <Typography.Text type="secondary">昵称</Typography.Text>
            <div className="profile-name-row">
              <Typography.Title level={3} className="profile-display-name">
                {displayName}
              </Typography.Title>
              <Button
                type="primary"
                icon={<Pencil size={16} />}
                onClick={() => {
                  form.setFieldsValue({ display_name: displayName });
                  setNicknameOpen(true);
                }}
              >
                修改
              </Button>
            </div>
          </div>
        </div>
      </Card>

      <Modal
        rootClassName="center-security-modal"
        title="修改昵称"
        open={nicknameOpen}
        onCancel={() => setNicknameOpen(false)}
        footer={null}
        destroyOnHidden
        width={420}
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={{ display_name: displayName }}
          onFinish={(values) => updateProfile.mutate({ display_name: values.display_name.trim() })}
        >
          <Form.Item
            label="昵称"
            name="display_name"
            rules={[
              { required: true, message: "请输入昵称" },
              { min: 2, message: "昵称至少 2 个字符" },
              { max: 30, message: "昵称最多 30 个字符" }
            ]}
          >
            <Input size="large" placeholder="请输入昵称" />
          </Form.Item>
          <Button type="primary" size="large" block htmlType="submit" icon={<Save size={16} />} loading={updateProfile.isPending}>
            保存昵称
          </Button>
        </Form>
      </Modal>

      <div className="metric-grid">
        <Card className="metric-card">
          <Statistic title="总余额" value={wallet.data?.balance_coins || 0} formatter={(value) => points(Number(value))} />
        </Card>
        <Card className="metric-card">
          <Statistic title="可用余额" value={wallet.data?.available_coins || 0} formatter={(value) => points(Number(value))} />
        </Card>
        <Card className="metric-card">
          <Statistic title="冻结余额" value={wallet.data?.reserved_coins || 0} formatter={(value) => points(Number(value))} />
        </Card>
        <Card className="metric-card">
          <Statistic title="当前用户" value={displayName} />
        </Card>
      </div>

      <div className="center-module-grid">
        <Card title="已订购套餐" className="record-list-card">
          <SubscriptionCardList
            loading={subscriptions.isPending || entitlements.isPending}
            items={subscriptions.data?.items || []}
            entitlements={entitlements.data?.items || []}
          />
        </Card>

        <Card title="钱包流水" className="record-list-card">
          <WalletLedgerCardList loading={ledger.isPending} items={ledger.data?.items || []} />
        </Card>

        <Card title="使用记录" className="record-list-card">
          <UsageActivityCardList
            loading={usage.isPending || entitlementLedger.isPending}
            usageItems={usage.data?.items || []}
            entitlementItems={entitlementLedger.data?.items || []}
          />
        </Card>
      </div>
    </div>
  );
}

function avatarSrc(value?: string, version?: string) {
  if (!value) return "/default-avatar.svg";
  if (!value.includes("/api/v1/users/avatar/")) return value;
  const joiner = value.includes("?") ? "&" : "?";
  return `${value}${joiner}v=${encodeURIComponent(version || "")}`;
}

function WalletLedgerCardList({ loading, items }: { loading: boolean; items: WalletLedgerItem[] }) {
  if (loading) return <div className="record-list-placeholder">加载中...</div>;
  if (!items.length) return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无钱包流水" />;

  return (
    <div className="record-list">
      {items.map((item) => {
        const direction = item.direction.toLowerCase();
        const negative = ["debit", "out", "consume", "deduct", "expense"].includes(direction) || item.amount_coins < 0;
        const positive = !negative;
        const amountText = `${positive ? "+" : "-"}${points(Math.abs(item.amount_coins))}`;
        return (
          <div className="record-row" key={item.id}>
            <div className="record-main">
              <div className="record-title">{ledgerTitle(item)}</div>
              <div className="record-meta">{time(item.created_at)}</div>
            </div>
            <div className="record-side">
              <div className={`record-amount ${positive ? "record-positive" : "record-negative"}`}>{amountText}</div>
              <div className="record-sub">余额：{points(item.balance_after)}</div>
            </div>
          </div>
        );
      })}
    </div>
  );
}

function SubscriptionCardList({
  loading,
  items,
  entitlements
}: {
  loading: boolean;
  items: SubscriptionItem[];
  entitlements: EntitlementItem[];
}) {
  if (loading) return <div className="record-list-placeholder">加载中...</div>;
  if (!items.length) return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无订购套餐" />;

  return (
    <div className="subscription-list">
      {items.map((item) => {
        const related = entitlements.filter((entry) => entry.app_code === item.app_code);
        const quota = related.reduce((sum, entry) => sum + entry.quota, 0);
        const used = related.reduce((sum, entry) => sum + entry.used + entry.reserved, 0);
        const percent = quota > 0 ? Math.min(100, Math.round((used / quota) * 100)) : 0;
        return (
          <div className="subscription-card" key={item.id}>
            <div className="subscription-head">
              <div className="subscription-title">{item.app_name || item.app_code}-{item.plan_name}</div>
              <UserStatusBadge value={item.status} />
            </div>
            <div className="subscription-progress-line">
              <span>用量进度：{quota > 0 ? `${coins(used)} / ${coins(quota)}` : "暂无用量"}</span>
              {quota > 0 ? <span>{percent}%</span> : null}
            </div>
            <Progress percent={percent} size="small" showInfo={false} />
            <div className="subscription-meta">有效期至：{date(item.ends_at)}</div>
          </div>
        );
      })}
    </div>
  );
}

function UsageActivityCardList({
  loading,
  usageItems,
  entitlementItems
}: {
  loading: boolean;
  usageItems: UsageRequestItem[];
  entitlementItems: EntitlementLedgerItem[];
}) {
  const activities = useMemo(() => {
    const usageActivities = usageItems.map((item) => ({
      id: `usage-${item.id}`,
      time: item.created_at,
      title: `${item.app_name || item.app_code} AI 调用`,
      meta: [item.model_alias ? `模型：${item.model_alias}` : "", item.actual_total_tokens ? `Tokens：${coins(item.actual_total_tokens)}` : ""]
        .filter(Boolean)
        .join(" · "),
      amount: item.charged_coins > 0 ? `扣除 ${points(item.charged_coins)}` : "未扣点",
      tone: item.status === "committed" ? "record-negative" : "",
      status: usageStatusText(item.status)
    }));
    const entitlementActivities = entitlementItems.map((item) => ({
      id: `entitlement-${item.id}`,
      time: item.created_at,
      title: `${item.app_name || item.app_code} 套餐用量`,
      meta: [entitlementActionText(item), item.entitlement_code].filter(Boolean).join(" · "),
      amount: entitlementAmountText(item),
      tone: item.direction === "consume" ? "record-negative" : "record-positive",
      status: ""
    }));
    return [...usageActivities, ...entitlementActivities]
      .sort((left, right) => new Date(right.time).getTime() - new Date(left.time).getTime())
      .slice(0, 12);
  }, [entitlementItems, usageItems]);

  if (loading) return <div className="record-list-placeholder">加载中...</div>;
  if (!activities.length) return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无使用记录" />;

  return (
    <div className="record-list">
      {activities.map((item) => (
        <div className="record-row" key={item.id}>
          <div className="record-main">
            <div className="record-title">{item.title}</div>
            <div className="record-meta">{item.meta || time(item.time)}</div>
          </div>
          <div className="record-side">
            <div className={`record-amount ${item.tone}`}>{item.amount}</div>
            <div className="record-sub">{item.status || time(item.time)}</div>
          </div>
        </div>
      ))}
    </div>
  );
}

function ledgerTitle(item: WalletLedgerItem) {
  const reasonLabels: Record<string, string> = {
    recharge: "充值",
    credit: "入账",
    debit: "扣点",
    usage: "调用扣点",
    admin_adjustment: "手工调整",
    reservation: "预冻结",
    reservation_release: "释放冻结",
    reserve: "预冻结",
    release: "释放冻结",
    refund: "退款"
  };
  const reason = reasonLabels[normalizeLedgerText(item.reason)] || humanizeLedgerText(item.reason) || "钱包变动";
  const remark = humanizeLedgerText(item.remark.replace(/^sample:/, "").trim());
  if (remark) return `${reason}-${remark}`;
  return reason;
}

function normalizeLedgerText(value?: string) {
  return (value || "").trim().toLowerCase().replace(/[\s-]+/g, "_");
}

function humanizeLedgerText(value?: string) {
  const normalized = normalizeLedgerText(value);
  if (!normalized) return "";
  const labels: Record<string, string> = {
    wechat: "微信",
    wechat_pay: "微信支付",
    wechat_payment: "微信支付",
    wechat_recharge: "微信支付",
    wechat_jsapi: "微信支付",
    wechat_native: "微信支付",
    native: "扫码支付",
    jsapi: "微信支付",
    recharge: "充值",
    usage: "调用扣点",
    admin_adjustment: "手工调整",
    reservation: "预冻结",
    reservation_release: "释放冻结",
    reserve: "预冻结",
    release: "释放冻结",
    refund: "退款"
  };
  return labels[normalized] || value?.trim() || "";
}

function UserStatusBadge({ value }: { value?: string }) {
  return <span className={`user-status-badge user-status-${value || "default"}`}>{subscriptionStatusText(value)}</span>;
}

function subscriptionStatusText(value?: string) {
  const labels: Record<string, string> = {
    active: "生效中",
    expired: "已过期",
    cancelled: "已取消",
    canceled: "已取消"
  };
  return labels[value || ""] || "未知";
}

function usageStatusText(value?: string) {
  const labels: Record<string, string> = {
    committed: "已完成",
    authorized: "处理中",
    pending: "待处理",
    cancelled: "已取消",
    canceled: "已取消",
    failed: "失败"
  };
  return labels[value || ""] || "处理中";
}

function entitlementActionText(item: EntitlementLedgerItem) {
  const labels: Record<string, string> = {
    consume: "已使用套餐额度",
    reserve: "额度预留中",
    release: "已释放预留额度",
    reset: "本月额度已刷新"
  };
  return labels[item.direction] || "套餐额度变动";
}

function entitlementAmountText(item: EntitlementLedgerItem) {
  const unitLabels: Record<string, string> = {
    tokens: "Tokens",
    images: "张",
    video_seconds: "秒",
    business_units: "次"
  };
  const prefix = item.direction === "consume" || item.direction === "reserve" ? "-" : "+";
  return `${prefix}${coins(item.amount)}${unitLabels[item.unit] || ""}`;
}
