import { useQuery, useQueryClient } from "@tanstack/react-query";
import { App, Button, Space, Spin, Tag, Typography } from "antd";
import { LogIn, LogOut, UserPlus } from "lucide-react";
import { useLocation } from "react-router-dom";
import { api, loginWithCasdoor, logout, registerAccount } from "../api/client";

export function AccountHeader() {
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const location = useLocation();
  const me = useQuery({ queryKey: ["me"], queryFn: api.me, retry: false });

  const submitLogout = async () => {
    await logout().catch(() => undefined);
    localStorage.setItem("koffy-auth-updated", String(Date.now()));
    queryClient.clear();
    message.success("已退出");
    window.setTimeout(() => window.location.reload(), 150);
  };

  if (me.isPending) {
    return <Spin size="small" />;
  }

  if (me.isError) {
    if (location.pathname.startsWith("/admin")) {
      return (
        <Button type="primary" icon={<LogIn size={16} />} onClick={loginWithCasdoor}>
          登录
        </Button>
      );
    }

    return (
      <Space size={8} wrap>
        <Button type="primary" icon={<LogIn size={16} />} onClick={loginWithCasdoor}>
          登录
        </Button>
        <Button icon={<UserPlus size={16} />} onClick={registerAccount}>
          注册
        </Button>
      </Space>
    );
  }

  const displayName = me.data.display_name || me.data.phone || "用户";
  const avatarURL = avatarSrc(me.data.avatar_url, me.data.updated_at);

  return (
    <Space size={12} wrap>
      <div className="account-name">
        <img className="account-avatar" src={avatarURL} alt="" />
        <Typography.Text strong>{displayName}</Typography.Text>
        {me.data.is_admin ? <Tag color="blue">管理员</Tag> : null}
      </div>
      <Button icon={<LogOut size={16} />} onClick={submitLogout}>
        退出
      </Button>
    </Space>
  );
}

function avatarSrc(value?: string, version?: string) {
  if (!value) return "/default-avatar.svg";
  if (!value.includes("/api/v1/users/avatar/")) return value;
  const joiner = value.includes("?") ? "&" : "?";
  return `${value}${joiner}v=${encodeURIComponent(version || "")}`;
}
