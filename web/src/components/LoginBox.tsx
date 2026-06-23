import { useMutation, useQuery } from "@tanstack/react-query";
import { App, Button, Card, Form, Input, Space } from "antd";
import { KeyRound, LogIn, UserPlus } from "lucide-react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api/client";

type LoginMode = "wechat" | "phone";

type LoginFormValues = {
  account: string;
  password: string;
};

type LoginBoxProps = {
  returnTo: string;
};

export function LoginBox({ returnTo }: LoginBoxProps) {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const [form] = Form.useForm<LoginFormValues>();
  const [loginMode, setLoginMode] = useState<LoginMode>("wechat");

  const submit = useMutation({
    mutationFn: (values: LoginFormValues) =>
      api.loginWithPassword({
        account: values.account,
        password: values.password,
        return_to: returnTo
      }),
    onSuccess: () => {
      localStorage.setItem("koffy-auth-updated", String(Date.now()));
      message.success("登录成功");
    },
    onSettled: (data) => {
      if (data?.redirect_to) {
        window.location.href = data.redirect_to;
      }
    },
    onError: (error) => {
      message.error(error instanceof Error ? error.message : "登录失败，请检查账号和密码");
    }
  });

  const goRegister = () => {
    navigate(`/register?return_to=${encodeURIComponent(returnTo)}`);
  };

  return (
    <Card className={`register-card login-card login-card-${loginMode}`}>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <div className="login-logo">
          <img src="/api/v1/branding/logo?area=center" alt="Koffy" />
        </div>

        <div className="login-tabs" role="tablist" aria-label="登录方式">
          <button type="button" className={`login-tab ${loginMode === "wechat" ? "active" : ""}`} onClick={() => setLoginMode("wechat")}>
            微信登录
          </button>
          <button type="button" className={`login-tab ${loginMode === "phone" ? "active" : ""}`} onClick={() => setLoginMode("phone")}>
            手机登录
          </button>
        </div>

        <div className="login-panel-frame">
          {loginMode === "wechat" ? (
            <div className="wechat-login-panel">
              <WeChatLoginEmbed returnTo={returnTo} />
            </div>
          ) : (
            <Form className="phone-login-form" form={form} layout="vertical" requiredMark={false} onFinish={(values) => submit.mutate(values)}>
              <Form.Item
                label="手机号"
                name="account"
                rules={[
                  { required: true, message: "请输入手机号" },
                  { whitespace: true, message: "请输入手机号" }
                ]}
              >
                <Input size="large" placeholder="请输入手机号" autoComplete="username" />
              </Form.Item>

              <Form.Item label="密码" name="password" rules={[{ required: true, message: "请输入密码" }]}>
                <Input.Password size="large" placeholder="请输入密码" autoComplete="current-password" />
              </Form.Item>

              <Button type="primary" size="large" htmlType="submit" block icon={<LogIn size={16} />} loading={submit.isPending}>
                登录
              </Button>
            </Form>
          )}
        </div>

        <div className={`register-actions login-actions ${loginMode === "wechat" ? "login-actions-empty" : ""}`}>
          {loginMode === "phone" ? (
            <>
              <Button type="link" icon={<KeyRound size={16} />} onClick={() => navigate(`/forgot-password?return_to=${encodeURIComponent(returnTo)}`)}>
                忘记密码
              </Button>
              <Button type="link" icon={<UserPlus size={16} />} onClick={goRegister}>
                注册账户
              </Button>
            </>
          ) : null}
        </div>
      </Space>
    </Card>
  );
}

function WeChatLoginEmbed({ returnTo }: { returnTo: string }) {
  const config = useQuery({
    queryKey: ["wechat-widget-config", returnTo],
    queryFn: () => api.wechatWidgetConfig({ action: "login", return_to: returnTo }),
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    staleTime: Infinity
  });

  if (config.isPending) {
    return <div className="wechat-login-iframe" aria-hidden="true" />;
  }

  const authURL = config.data?.auth_url || `/api/v1/auth/wechat/start?action=login&return_to=${encodeURIComponent(returnTo)}`;
  const openQuickLogin = () => {
    window.location.href = authURL;
  };

  if (config.isError) {
    return (
      <WeChatQuickLoginView onQuickLogin={openQuickLogin} />
    );
  }

  if (config.data?.app_type === "official") {
    return (
      <WeChatQuickLoginView onQuickLogin={openQuickLogin} />
    );
  }

  return (
    <iframe
      className="wechat-login-iframe"
      title="微信登录"
      src={wechatIframeURL(authURL, config.data?.stylelite)}
      scrolling="no"
      allow="local-network-access"
      onLoad={(event) => {
        try {
          const href = event.currentTarget.contentWindow?.location.href;
          if (!href) return;
          const target = new URL(href);
          if (target.origin !== window.location.origin) return;
          if (target.href === window.location.href) return;
          if (target.pathname === "/api/v1/auth/wechat/start") return;
          if (target.pathname === "/api/v1/auth/wechat/callback") return;
          window.location.href = `${target.pathname}${target.search}${target.hash}`;
        } catch {
          // Cross-origin while the WeChat widget is active.
        }
      }}
    />
  );
}

function wechatIframeURL(authURL: string, stylelite?: string) {
  try {
    const url = new URL(authURL, window.location.origin);
    url.searchParams.set("self_redirect", "true");
    url.searchParams.set("stylelite", stylelite || "1");
    return url.toString();
  } catch {
    const joiner = authURL.includes("?") ? "&" : "?";
    return `${authURL}${joiner}self_redirect=true&stylelite=${encodeURIComponent(stylelite || "1")}`;
  }
}

function WeChatQuickLoginView({ onQuickLogin }: { onQuickLogin: () => void }) {
  return (
    <div className="wechat-quick-login">
      <button type="button" className="wechat-login-fallback" onClick={onQuickLogin} aria-label="微信快捷登录">
        <WeChatClickLoginArt />
      </button>
    </div>
  );
}

function WeChatClickLoginArt() {
  return <img src="/wechat-login-mark.png" alt="" />;
}
