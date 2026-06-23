import { useMutation } from "@tanstack/react-query";
import { App, Button, Card, Form, Input, Result, Space, Typography } from "antd";
import { ArrowLeft, LogIn, ShieldCheck } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { api } from "../api/client";
import { HumanCheck } from "../components/HumanCheck";
import { setBrandingFavicon } from "../components/branding";

type ForgotPasswordValues = {
  phone: string;
  code: string;
  password: string;
  confirm_password: string;
  human_token: string;
};
const MAINLAND_PHONE_PATTERN = /^1[3-9]\d{9}$/;

export function ForgotPasswordPage() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const [form] = Form.useForm<ForgotPasswordValues>();
  const [sent, setSent] = useState(false);
  const [cooldown, setCooldown] = useState(0);
  const [done, setDone] = useState(false);
  const [debugCode, setDebugCode] = useState("");
  const returnTo = useMemo(() => params.get("return_to") || "/center", [params]);

  useEffect(() => {
    document.title = "用户中心";
    setBrandingFavicon("center");
  }, []);

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = window.setTimeout(() => setCooldown((value) => Math.max(0, value - 1)), 1000);
    return () => window.clearTimeout(timer);
  }, [cooldown]);

  useEffect(() => {
    if (!sent || cooldown > 0) return;
    setSent(false);
    form.setFieldsValue({ human_token: "" });
  }, [cooldown, form, sent]);

  const sendCode = useMutation({
    mutationFn: ({ phone, human_token }: { phone: string; human_token: string }) =>
      api.sendResetPasswordCode({ country_code: "+86", phone, human_token }),
    onSuccess: (data) => {
      setSent(true);
      setCooldown(60);
      setDebugCode(data.debug_code || "");
      message.success("验证码已发送");
    },
    onError: (error) => {
      form.setFieldsValue({ human_token: "" });
      message.error(error instanceof Error ? error.message : "验证码发送失败");
    },
    meta: { skipGlobalError: true }
  });

  const handleSendCode = () => {
    const rawPhone = form.getFieldValue("phone");
    const phone = typeof rawPhone === "string" ? rawPhone.trim() : "";
    if (!MAINLAND_PHONE_PATTERN.test(phone)) {
      form.setFields([{ name: "phone", errors: ["请输入正确的手机号"] }]);
      message.warning("请输入正确的手机号");
      return;
    }
    form.setFields([{ name: "phone", errors: [] }]);
    const humanToken = form.getFieldValue("human_token");
    if (!humanToken) {
      message.warning("请先完成“安全验证”。");
      return;
    }
    sendCode.mutate({ phone, human_token: humanToken });
  };

  const submit = useMutation({
    mutationFn: (values: ForgotPasswordValues) =>
      api.resetPassword({
        country_code: "+86",
        phone: values.phone,
        code: values.code,
        password: values.password,
        confirm_password: values.confirm_password
      }),
    onSuccess: () => {
      setDone(true);
      message.success("密码已重置");
    },
    onError: (error) => message.error(error instanceof Error ? error.message : "密码重置失败")
  });

  if (done) {
    return (
      <div className="auth-page">
        <Result
          status="success"
          title="密码已重置"
          extra={
            <Button type="primary" icon={<LogIn size={16} />} onClick={() => navigate(`/login?return_to=${encodeURIComponent(returnTo)}`)}>
              返回登录
            </Button>
          }
        />
      </div>
    );
  }

  return (
    <div className="auth-page">
      <Card className="register-card">
        <Space direction="vertical" size={18} style={{ width: "100%" }}>
          <div className="register-head">
            <img src="/api/v1/branding/logo?area=center" alt="Koffy" />
            <div>
              <Typography.Title level={3} style={{ margin: 0 }}>
                忘记密码
              </Typography.Title>
              <Typography.Text type="secondary">通过手机号验证码重置登录密码。</Typography.Text>
            </div>
          </div>

          <Form form={form} layout="vertical" requiredMark={false} autoComplete="off" onFinish={(values) => submit.mutate(values)}>
            <Form.Item
              label="手机号"
              name="phone"
              rules={[
                { required: true, message: "请输入手机号" },
                { pattern: MAINLAND_PHONE_PATTERN, message: "请输入正确的手机号" }
              ]}
            >
              <Input size="large" addonBefore="+86" inputMode="numeric" maxLength={11} autoComplete="tel-national" placeholder="请输入手机号" />
            </Form.Item>

            <Form.Item name="human_token" className="security-captcha-field">
              <HumanCheck />
            </Form.Item>

            <Form.Item label="验证码" required>
              <Space.Compact style={{ width: "100%" }}>
                <Form.Item
                  name="code"
                  noStyle
                  rules={[
                    { required: true, message: "请输入验证码" },
                    { pattern: /^\d{6}$/, message: "验证码为 6 位数字" }
                  ]}
                >
                  <Input
                    size="large"
                    inputMode="numeric"
                    maxLength={6}
                    autoComplete="off"
                    name="sms_verification_code"
                    prefix={<ShieldCheck size={16} />}
                  />
                </Form.Item>
                <Button size="large" htmlType="button" loading={sendCode.isPending} disabled={cooldown > 0} onClick={handleSendCode}>
                  {cooldown > 0 ? `${cooldown}秒后重发` : "发送验证码"}
                </Button>
              </Space.Compact>
            </Form.Item>
            {debugCode ? <Typography.Text type="secondary">本地测试验证码：{debugCode}</Typography.Text> : null}

            <input className="browser-autofill-decoy" type="text" name="username" autoComplete="username" tabIndex={-1} aria-hidden="true" />
            <Form.Item
              label="新密码"
              name="password"
              rules={[
                { required: true, message: "请输入新密码" },
                { min: 8, message: "密码长度必须至少为 8 个字符" },
                { pattern: /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).+$/, message: "密码必须包含至少一个大写字母、一个小写字母和一个数字" }
              ]}
            >
              <Input.Password size="large" placeholder="至少 8 位" autoComplete="new-password" />
            </Form.Item>

            <Form.Item
              label="确认新密码"
              name="confirm_password"
              dependencies={["password"]}
              rules={[
                { required: true, message: "请再次输入新密码" },
                ({ getFieldValue }) => ({
                  validator(_, value) {
                    return !value || getFieldValue("password") === value
                      ? Promise.resolve()
                      : Promise.reject(new Error("两次输入的密码不一致"));
                  }
                })
              ]}
            >
              <Input.Password size="large" placeholder="再次输入新密码" autoComplete="new-password" />
            </Form.Item>

            <Button type="primary" size="large" htmlType="submit" block loading={submit.isPending}>
              重置密码
            </Button>
          </Form>

          <div className="register-actions">
            <Button type="link" icon={<ArrowLeft size={16} />} onClick={() => navigate(`/login?return_to=${encodeURIComponent(returnTo)}`)}>
              返回登录
            </Button>
          </div>
        </Space>
      </Card>
    </div>
  );
}
