import { useQuery } from "@tanstack/react-query";
import { Alert, Button, Space, Typography } from "antd";
import { CheckCircle2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { api } from "../api/client";

declare global {
  interface Window {
    turnstile?: {
      render: (el: HTMLElement, options: Record<string, unknown>) => string;
      reset: (id?: string) => void;
    };
    hcaptcha?: {
      render: (el: HTMLElement, options: Record<string, unknown>) => string;
      reset: (id?: string) => void;
    };
    TencentCaptcha?: new (...args: unknown[]) => { show: () => void; destroy?: () => void };
  }
}

type TencentCaptchaResult = {
  ret: number;
  ticket?: string | null;
  randstr?: string | null;
  CaptchaAppId?: string;
  errorCode?: number;
  errorMessage?: string;
};

type HumanCheckProps = {
  value?: string;
  onChange?: (token: string) => void;
};

export function HumanCheck({ value, onChange }: HumanCheckProps) {
  const config = useQuery({ queryKey: ["auth-config"], queryFn: api.authConfig, staleTime: 5 * 60 * 1000 });
  const containerRef = useRef<HTMLDivElement>(null);
  const widgetRef = useRef<unknown>(null);
  const onChangeRef = useRef(onChange);
  const previousValueRef = useRef(value);
  const [tencentError, setTencentError] = useState("");
  const [tencentReady, setTencentReady] = useState(false);
  const [tencentRenderKey, setTencentRenderKey] = useState(0);
  const provider = config.data?.provider || "none";
  const enabled = Boolean(config.data?.enabled);
  const resetTencentCaptcha = () => {
    onChange?.("");
    const widget = widgetRef.current as { destroy?: () => void } | null;
    widget?.destroy?.();
    widgetRef.current = null;
    if (containerRef.current) {
      containerRef.current.innerHTML = "";
    }
    setTencentError("");
    setTencentRenderKey((current) => current + 1);
  };

  useEffect(() => {
    onChangeRef.current = onChange;
  }, [onChange]);

  useEffect(() => {
    if (!config.data) return;
    if (!enabled && value !== "captcha-disabled") {
      onChangeRef.current?.("captcha-disabled");
    }
  }, [config.data, enabled, value]);

  useEffect(() => {
    const previousValue = previousValueRef.current;
    previousValueRef.current = value;
    if (provider !== "tencent" || !enabled || !previousValue || value) return;
    const widget = widgetRef.current as { destroy?: () => void } | null;
    widget?.destroy?.();
    widgetRef.current = null;
    if (containerRef.current) {
      containerRef.current.innerHTML = "";
    }
    setTencentError("");
    setTencentRenderKey((current) => current + 1);
  }, [enabled, provider, value]);

  useEffect(() => {
    if (!config.data || !enabled || provider === "tencent" || !containerRef.current || widgetRef.current) return;
    const sitekey = config.data.site_key;
    const renderWidget = () => {
      if (!containerRef.current || widgetRef.current) return;
      const callback = (token: string) => onChangeRef.current?.(token);
      if ((provider === "turnstile" || provider === "cloudflare") && window.turnstile) {
        widgetRef.current = window.turnstile.render(containerRef.current, { sitekey, callback });
      }
      if (provider === "hcaptcha" && window.hcaptcha) {
        widgetRef.current = window.hcaptcha.render(containerRef.current, { sitekey, callback });
      }
    };
    const script = document.createElement("script");
    script.async = true;
    script.defer = true;
    script.onload = renderWidget;
    script.src =
      provider === "hcaptcha"
        ? "https://js.hcaptcha.com/1/api.js?render=explicit"
        : "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";
    document.head.appendChild(script);
    return () => {
      script.remove();
    };
  }, [config.data, enabled, provider]);

  useEffect(() => {
    if (!config.data || !enabled || provider !== "tencent") return;
    const createTencentCaptcha = () => {
      if (!config.data?.site_key || !window.TencentCaptcha || widgetRef.current) return;
      setTencentError("");
      try {
        widgetRef.current = new window.TencentCaptcha(
          config.data.site_key,
          (result: TencentCaptchaResult) => {
            if (result.ret === 0 && result.ticket && result.randstr) {
              onChangeRef.current?.(
                JSON.stringify({
                  ticket: result.ticket,
                  randstr: result.randstr,
                  captcha_app_id: result.CaptchaAppId || config.data?.site_key || ""
                })
              );
              return;
            }
            onChangeRef.current?.("");
            if (result.ret === 2) {
              setTencentError("已取消安全验证");
              return;
            }
            setTencentError(result.errorMessage || "安全验证未通过，请重新验证。");
          },
          { userLanguage: "zh-cn" }
        );
        setTencentReady(true);
      } catch {
        setTencentError("腾讯云验证码启动失败，请刷新后重试。");
      }
    };

    if (window.TencentCaptcha) {
      createTencentCaptcha();
      return;
    }

    const scriptId = "tencent-captcha-sdk";
    const existingScript = document.getElementById(scriptId) as HTMLScriptElement | null;
    if (existingScript) {
      existingScript.addEventListener("load", createTencentCaptcha, { once: true });
      existingScript.addEventListener("error", () => setTencentError("腾讯云验证码加载失败，请刷新后重试。"), { once: true });
      return;
    }

    const script = document.createElement("script");
    script.id = scriptId;
    script.async = true;
    script.defer = true;
    script.onload = createTencentCaptcha;
    script.onerror = () => setTencentError("腾讯云验证码加载失败，请刷新后重试。");
    script.src = "https://turing.captcha.qcloud.com/TJCaptcha.js";
    document.head.appendChild(script);
    return () => {
      const widget = widgetRef.current as { destroy?: () => void } | null;
      widget?.destroy?.();
      widgetRef.current = null;
      setTencentReady(false);
    };
  }, [config.data, enabled, provider, tencentRenderKey]);

  const showTencentCaptcha = () => {
    setTencentError("");
    const widget = widgetRef.current as { show?: () => void } | null;
    if (!widget?.show) {
      setTencentError("腾讯云验证码尚未加载完成，请稍后再试。");
      return;
    }
    widget.show();
  };

  if (config.isPending) {
    return <Typography.Text type="secondary">正在加载安全验证...</Typography.Text>;
  }

  if (!enabled) {
    return <span className="human-check-disabled" aria-hidden="true" />;
  }

  if (!config.data?.site_key) {
    return <Alert type="warning" showIcon message="安全验证站点配置缺失，请联系管理员。" />;
  }

  if (provider === "tencent") {
    if (value) {
      return (
        <div className="tencent-captcha-success">
          <CheckCircle2 size={28} />
          <span>验证成功</span>
          <Button type="link" size="small" onClick={resetTencentCaptcha}>
            重新验证
          </Button>
        </div>
      );
    }
    return (
      <Space direction="vertical" size={8} style={{ width: "100%" }}>
        <div ref={containerRef} />
        <Button size="large" block onClick={showTencentCaptcha} loading={!tencentReady}>
          安全验证
        </Button>
        {tencentError ? <Typography.Text type="danger">{tencentError}</Typography.Text> : null}
      </Space>
    );
  }

  if (provider !== "turnstile" && provider !== "cloudflare" && provider !== "hcaptcha") {
    return <Alert type="warning" showIcon message="当前人机验证服务暂不支持，请联系管理员。" />;
  }

  return (
    <Space direction="vertical" size={8} style={{ width: "100%" }}>
      <div ref={containerRef} />
      {!value ? <Typography.Text type="secondary">发送验证码前请先完成安全验证。</Typography.Text> : null}
    </Space>
  );
}
