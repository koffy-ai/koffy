import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { App, Button, Card, Form, Input, Modal, Space, Tag, Typography } from "antd";
import { KeyRound, Phone, ShieldCheck } from "lucide-react";
import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { api, startWeChatBind } from "../../api/client";
import { HumanCheck } from "../../components/HumanCheck";

type BindPhoneValues = {
  phone: string;
  human_token: string;
  code: string;
  password: string;
  confirm_password: string;
};

type ChangePasswordValues = {
  human_token: string;
  code: string;
  password: string;
  confirm_password: string;
};

type PhoneDialogMode = "bind" | "change";
const MAINLAND_PHONE_PATTERN = /^1[3-9]\d{9}$/;

export function AccountSecurityPage() {
  const { message, modal } = App.useApp();
  const queryClient = useQueryClient();
  const [params, setParams] = useSearchParams();
  const [phoneDialogMode, setPhoneDialogMode] = useState<PhoneDialogMode>("bind");
  const [phoneDialogOpen, setPhoneDialogOpen] = useState(false);
  const [passwordDialogOpen, setPasswordDialogOpen] = useState(false);
  const [bindCodeSent, setBindCodeSent] = useState(false);
  const [passwordCodeSent, setPasswordCodeSent] = useState(false);
  const [bindCooldown, setBindCooldown] = useState(0);
  const [passwordCooldown, setPasswordCooldown] = useState(0);
  const me = useQuery({ queryKey: ["me"], queryFn: api.me });
  const bindings = useQuery({ queryKey: ["auth-bindings"], queryFn: api.authBindings });
  const [bindForm] = Form.useForm<BindPhoneValues>();
  const [passwordForm] = Form.useForm<ChangePasswordValues>();

  useEffect(() => {
    const errorMessage = params.get("wechat_error");
    if (!errorMessage) return;
    if (params.get("wechat_error_code") === "wechat_already_bound") {
      modal.error({
        title: "微信绑定失败",
        content: "抱歉，该微信号已绑定其它账号，您可以直接使用该微信号登录另一个账号，或者使用另一个微信号绑定当前账号。",
        okText: "知道了"
      });
    } else {
      message.error(errorMessage);
    }
    const next = new URLSearchParams(params);
    next.delete("wechat_error");
    next.delete("wechat_error_code");
    setParams(next, { replace: true });
  }, [message, modal, params, setParams]);

  useEffect(() => {
    if (bindCooldown <= 0) return;
    const timer = window.setTimeout(() => setBindCooldown((value) => Math.max(0, value - 1)), 1000);
    return () => window.clearTimeout(timer);
  }, [bindCooldown]);

  useEffect(() => {
    if (!bindCodeSent || bindCooldown > 0) return;
    setBindCodeSent(false);
    bindForm.setFieldsValue({ human_token: "" });
  }, [bindCodeSent, bindCooldown, bindForm]);

  useEffect(() => {
    if (passwordCooldown <= 0) return;
    const timer = window.setTimeout(() => setPasswordCooldown((value) => Math.max(0, value - 1)), 1000);
    return () => window.clearTimeout(timer);
  }, [passwordCooldown]);

  useEffect(() => {
    if (!passwordCodeSent || passwordCooldown > 0) return;
    setPasswordCodeSent(false);
    passwordForm.setFieldsValue({ human_token: "" });
  }, [passwordCodeSent, passwordCooldown, passwordForm]);

  const openPhoneDialog = (mode: PhoneDialogMode) => {
    setPhoneDialogMode(mode);
    setBindCodeSent(false);
    setBindCooldown(0);
    bindForm.resetFields();
    setPhoneDialogOpen(true);
  };

  const closePhoneDialog = () => {
    setBindCodeSent(false);
    setBindCooldown(0);
    setPhoneDialogOpen(false);
    bindForm.resetFields();
  };

  const openPasswordDialog = () => {
    setPasswordCodeSent(false);
    setPasswordCooldown(0);
    passwordForm.resetFields();
    setPasswordDialogOpen(true);
  };

  const closePasswordDialog = () => {
    setPasswordCodeSent(false);
    setPasswordCooldown(0);
    setPasswordDialogOpen(false);
    passwordForm.resetFields();
  };

  const sendBindCode = useMutation({
    mutationFn: ({ phone, human_token }: { phone: string; human_token: string }) =>
      api.sendBindPhoneCode({ country_code: "+86", phone, human_token }),
    onSuccess: (data) => {
      setBindCodeSent(true);
      setBindCooldown(60);
      message.success(data.debug_code ? `验证码已发送，本地验证码：${data.debug_code}` : "验证码已发送");
    },
    onError: (error) => {
      bindForm.setFieldsValue({ human_token: "" });
      message.warning(error instanceof Error ? error.message : "验证码发送失败");
    },
    meta: { skipGlobalError: true }
  });

  const bindPhone = useMutation({
    mutationFn: (values: BindPhoneValues) =>
      api.bindPhone({
        country_code: "+86",
        phone: values.phone,
        code: values.code,
        password: values.password,
        confirm_password: values.confirm_password
      }),
    onSuccess: () => {
      message.success(phoneDialogMode === "bind" ? "手机号已绑定" : "手机号已更换");
      closePhoneDialog();
      queryClient.invalidateQueries({ queryKey: ["me"] });
      queryClient.invalidateQueries({ queryKey: ["auth-bindings"] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : "手机号操作失败"),
    meta: { skipGlobalError: true }
  });

  const sendPasswordCode = useMutation({
    mutationFn: ({ human_token }: { human_token: string }) => api.sendChangePasswordCode({ human_token }),
    onSuccess: (data) => {
      setPasswordCodeSent(true);
      setPasswordCooldown(60);
      message.success(data.debug_code ? `验证码已发送，本地验证码：${data.debug_code}` : "验证码已发送");
    },
    onError: (error) => {
      passwordForm.setFieldsValue({ human_token: "" });
      message.warning(error instanceof Error ? error.message : "验证码发送失败");
    },
    meta: { skipGlobalError: true }
  });

  const changePassword = useMutation({
    mutationFn: (values: ChangePasswordValues) =>
      api.changePassword({
        code: values.code,
        password: values.password,
        confirm_password: values.confirm_password
      }),
    onSuccess: () => {
      message.success("密码已修改");
      closePasswordDialog();
    },
    onError: (error) => message.error(error instanceof Error ? error.message : "密码修改失败"),
    meta: { skipGlobalError: true }
  });

  const unbindWeChat = useMutation({
    mutationFn: api.unbindWeChat,
    onSuccess: () => {
      message.success("微信已解绑");
      queryClient.invalidateQueries({ queryKey: ["auth-bindings"] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : "微信解绑失败"),
    meta: { skipGlobalError: true }
  });

  const handleSendBindCode = () => {
    const rawPhone = bindForm.getFieldValue("phone");
    const phone = typeof rawPhone === "string" ? rawPhone.trim() : "";
    if (!MAINLAND_PHONE_PATTERN.test(phone)) {
      bindForm.setFields([{ name: "phone", errors: ["请输入正确的手机号"] }]);
      message.warning("请输入正确的手机号");
      return;
    }
    bindForm.setFields([{ name: "phone", errors: [] }]);

    const humanToken = bindForm.getFieldValue("human_token");
    if (!humanToken) {
      message.warning("请先完成“安全验证”。");
      return;
    }
    sendBindCode.mutate({ phone, human_token: humanToken });
  };

  const handleSendPasswordCode = () => {
    const humanToken = passwordForm.getFieldValue("human_token");
    if (!humanToken) {
      message.warning("请先完成“安全验证”。");
      return;
    }
    sendPasswordCode.mutate({ human_token: humanToken });
  };

  const phoneBound = Boolean(me.data?.phone || bindings.data?.phone_bound);
  const wechatBound = Boolean(bindings.data?.wechat_bound);
  const wechatName = bindings.data?.wechat_nickname || "微信账号";
  const wechatAvatarURL = bindings.data?.wechat_avatar_url || "";

  const handleUnbindWeChat = () => {
    modal.confirm({
      rootClassName: "center-security-modal",
      title: "解绑微信？",
      content: "解绑后将不能使用该微信直接登录当前账号。",
      okText: "解绑",
      cancelText: "取消",
      okButtonProps: { loading: unbindWeChat.isPending },
      onOk: () => unbindWeChat.mutateAsync()
    });
  };

  return (
    <div className="page-stack account-security-page">
      <div className="page-title-row">
        <div>
          <h1 className="page-title">账号安全</h1>
          <Typography.Text type="secondary">管理登录方式，保障账号和资产安全。</Typography.Text>
        </div>
      </div>

      <div className="security-card-grid">
        <Card className="security-card">
          <div className="security-item">
            <div className="security-icon security-phone-icon">
              <Phone size={22} />
            </div>
            <div className="security-main">
              <div className="security-title-row">
                <Typography.Title level={4}>手机号</Typography.Title>
                {phoneBound ? (
                  <Tag color="green">已绑定</Tag>
                ) : (
                  <Button type="primary" onClick={() => openPhoneDialog("bind")}>
                    去绑定
                  </Button>
                )}
              </div>
              {phoneBound ? (
                <div className="security-phone-bound">
                  <Typography.Text className="security-value">{me.data?.phone || "已绑定"}</Typography.Text>
                  <div className="security-action-row">
                    <Button onClick={() => openPhoneDialog("change")}>更换手机号</Button>
                    <Button icon={<KeyRound size={16} />} onClick={openPasswordDialog}>
                      更改密码
                    </Button>
                  </div>
                </div>
              ) : (
                <Typography.Text type="secondary">绑定后可用于登录和找回密码。</Typography.Text>
              )}
            </div>
          </div>
        </Card>

        <Card className="security-card">
          <div className="security-item">
            <div className={`security-icon ${wechatBound && wechatAvatarURL ? "security-wechat-avatar" : "security-wechat-icon"}`}>
              {wechatBound && wechatAvatarURL ? <img src={wechatAvatarURL} alt="" /> : <WeChatLogo />}
            </div>
            <div className="security-main">
              <div className="security-title-row">
                <Typography.Title level={4}>{wechatBound ? wechatName : "微信"}</Typography.Title>
                {wechatBound ? (
                  <Button type="text" size="small" className="security-unbind-link" loading={unbindWeChat.isPending} onClick={handleUnbindWeChat}>
                    解绑
                  </Button>
                ) : (
                  <Button type="primary" className="wechat-bind-button" onClick={() => startWeChatBind()}>
                    去绑定
                  </Button>
                )}
              </div>
              <Typography.Text type="secondary">
                {wechatBound ? "可直接使用微信登录当前账号。" : "绑定后可直接使用微信登录"}
              </Typography.Text>
            </div>
          </div>
        </Card>
      </div>

      <Modal
        rootClassName="center-security-modal"
        title={phoneDialogMode === "bind" ? "绑定手机号" : "更换手机号"}
        open={phoneDialogOpen}
        onCancel={closePhoneDialog}
        footer={null}
        destroyOnHidden
        width={520}
      >
        <Form form={bindForm} layout="vertical" requiredMark={false} autoComplete="off" onFinish={(values) => bindPhone.mutate(values)}>
          <PhoneFormFields cooldown={bindCooldown} sendCodePending={sendBindCode.isPending} onSendCode={handleSendBindCode} />
          <PasswordFields />
          <Button type="primary" size="large" block htmlType="submit" loading={bindPhone.isPending}>
            {phoneDialogMode === "bind" ? "完成绑定" : "确认更换"}
          </Button>
        </Form>
      </Modal>

      <Modal
        rootClassName="center-security-modal"
        title="更改密码"
        open={passwordDialogOpen}
        onCancel={closePasswordDialog}
        footer={null}
        destroyOnHidden
        width={520}
      >
        <Form form={passwordForm} layout="vertical" requiredMark={false} autoComplete="off" onFinish={(values) => changePassword.mutate(values)}>
          <Form.Item name="human_token" className="security-captcha-field">
            <HumanCheck />
          </Form.Item>
          <Form.Item label="手机验证码" required>
            <Space.Compact style={{ width: "100%" }}>
              <Form.Item name="code" noStyle rules={[{ required: true, message: "请输入验证码" }]}>
                <Input
                  size="large"
                  inputMode="numeric"
                  maxLength={6}
                  autoComplete="off"
                  name="sms_verification_code"
                  prefix={<ShieldCheck size={16} />}
                />
              </Form.Item>
              <Button size="large" htmlType="button" loading={sendPasswordCode.isPending} disabled={passwordCooldown > 0} onClick={handleSendPasswordCode}>
                {passwordCooldown > 0 ? `${passwordCooldown}秒后重发` : "发送验证码"}
              </Button>
            </Space.Compact>
          </Form.Item>
          <PasswordFields />
          <Button type="primary" size="large" block icon={<KeyRound size={16} />} htmlType="submit" loading={changePassword.isPending}>
            确认修改
          </Button>
        </Form>
      </Modal>
    </div>
  );
}

function PhoneFormFields({ cooldown, sendCodePending, onSendCode }: { cooldown: number; sendCodePending: boolean; onSendCode: () => void }) {
  return (
    <>
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
      <Form.Item label="手机验证码" required>
        <Space.Compact style={{ width: "100%" }}>
          <Form.Item name="code" noStyle rules={[{ required: true, message: "请输入验证码" }]}>
            <Input
              size="large"
              inputMode="numeric"
              maxLength={6}
              autoComplete="off"
              name="sms_verification_code"
              prefix={<ShieldCheck size={16} />}
              placeholder="请输入验证码"
            />
          </Form.Item>
          <Button size="large" htmlType="button" loading={sendCodePending} disabled={cooldown > 0} onClick={onSendCode}>
            {cooldown > 0 ? `${cooldown}秒后重发` : "发送验证码"}
          </Button>
        </Space.Compact>
      </Form.Item>
    </>
  );
}

function WeChatLogo() {
  return (
    <svg viewBox="0 0 48 48" aria-hidden="true" focusable="false">
      <path
        fill="#07c160"
        d="M19.4 12.5c-7.1 0-12.9 4.7-12.9 10.6 0 3.3 1.8 6.2 4.7 8.1l-1 3.6 4.4-2.1c1.5.5 3.1.8 4.8.8 7.1 0 12.9-4.7 12.9-10.5S26.5 12.5 19.4 12.5z"
      />
      <path
        fill="#13d56b"
        d="M29.7 21.3c-6.2 0-11.2 4.1-11.2 9.1s5 9.1 11.2 9.1c1.5 0 2.9-.2 4.2-.7l3.8 1.8-.9-3.1c2.5-1.7 4.1-4.2 4.1-7.1 0-5-5-9.1-11.2-9.1z"
      />
      <circle cx="15.1" cy="21.4" r="1.7" fill="#fff" />
      <circle cx="23.5" cy="21.4" r="1.7" fill="#fff" />
      <circle cx="26.6" cy="29.2" r="1.4" fill="#fff" />
      <circle cx="34" cy="29.2" r="1.4" fill="#fff" />
    </svg>
  );
}

function PasswordFields() {
  return (
    <>
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
    </>
  );
}
