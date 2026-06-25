import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, App, Button, Card, Empty, Radio, Statistic, Typography } from "antd";
import { CreditCard } from "lucide-react";
import { QRCodeSVG } from "qrcode.react";
import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { api, startWeChatPayAuth } from "../../api/client";
import type { PaymentMethod, RechargeOrderItem, WeChatJSPayment } from "../../api/types";
import { cents, points, time } from "../../components/format";

const amountOptions = [10, 30, 50, 100, 200, 500];
const WECHAT_RECHARGE_PATH = "/center/recharge/";
const WECHAT_ALIPAY_NOTICE =
  "当前环境为微信内浏览，选择微信支付体验更佳。如果想在微信中使用支付宝支付，则需要在发起支付后复制显示的链接到手机浏览器中打开进行支付。直接在手机浏览器或者 PC 浏览器中使用支付宝支付体验会更加丝滑。";

type RechargeMutationVars = {
  nextAmountYuan: number;
  nextPaymentMethod: PaymentMethod;
  wechatPayCode?: string;
  payWindow?: Window | null;
};

declare global {
  interface Window {
    WeixinJSBridge?: {
      invoke: (
        name: "getBrandWCPayRequest",
        params: Omit<WeChatJSPayment, "channel">,
        callback: (result: { err_msg?: string }) => void
      ) => void;
    };
  }
}

export function RechargePage() {
  const { message, modal } = App.useApp();
  const queryClient = useQueryClient();
  const [params] = useSearchParams();
  const [amountYuan, setAmountYuan] = useState(30);
  const [paymentMethod, setPaymentMethod] = useState<PaymentMethod>("wechat");
  const [autoPayHandled, setAutoPayHandled] = useState(false);
  const [alipayWeChatNoticeShown, setAlipayWeChatNoticeShown] = useState(false);
  const isWeChatBrowser = /MicroMessenger/i.test(navigator.userAgent);
  const isMobileBrowser = /Android|iPhone|iPad|iPod|Mobile/i.test(navigator.userAgent);
  const orders = useQuery({
    queryKey: ["recharge-orders"],
    queryFn: () => api.rechargeOrders(50)
  });
  const mutation = useMutation({
    mutationFn: ({
      nextAmountYuan,
      nextPaymentMethod,
      wechatPayCode
    }: RechargeMutationVars) =>
      api.createRechargeOrder({
        amount_cents: nextAmountYuan * 100,
        channel: paymentChannel(nextPaymentMethod, isWeChatBrowser, isMobileBrowser),
        description: "点数充值",
        wechat_pay_code: wechatPayCode
      }),
    onSuccess: async (data, variables) => {
      await queryClient.invalidateQueries({ queryKey: ["recharge-orders"] });
      if (isWeChatBrowser && isWeChatJSPayment(data.payment)) {
        try {
          canonicalizeWeChatRechargeURL();
          await invokeWeChatPay(data.payment);
          await queryClient.invalidateQueries({ queryKey: ["recharge-orders"] });
          message.success("支付已提交，请稍后查看到账结果");
        } catch (error) {
          message.warning(error instanceof Error ? error.message : "微信支付未完成");
        }
        return;
      }
      const channel = typeof data.payment?.channel === "string" ? data.payment.channel : "";
      const payURL = typeof data.payment?.pay_url === "string" ? data.payment.pay_url : "";
      if (payURL) {
        if (isWeChatBrowser) {
          message.info("正在打开支付宝链接，请按微信提示复制到手机浏览器中继续支付");
          window.location.href = payURL;
          return;
        }
        const opened = openPaymentURL(payURL, variables.payWindow);
        if (opened) {
          message.success(channel === "alipay_wap" ? "已在新页面打开支付宝支付" : "已在新页面打开支付宝收银台");
        } else {
          modal.info({
            title: "打开支付宝支付",
            content: (
              <div className="qr-modal-content">
                <Typography.Text type="secondary">浏览器拦截了新页面，请点击下方按钮继续支付。</Typography.Text>
                <Button type="primary" href={payURL} target="_blank">
                  打开支付宝支付
                </Button>
              </div>
            ),
            okText: "关闭"
          });
        }
        return;
      }
      closePendingPaymentWindow(variables.payWindow);
      const codeURL = typeof data.payment?.code_url === "string" ? data.payment.code_url : "";
      if (codeURL) {
        modal.info({
          className: "center-qr-modal",
          icon: null,
          title: "微信扫码支付",
          width: 380,
          content: (
            <div className="qr-modal-content">
              <div className="qr-wrap wechat-qr-wrap">
                <QRCodeSVG value={codeURL} size={200} includeMargin />
              </div>
              <Typography.Text type="secondary">请使用微信扫码完成支付</Typography.Text>
              <Typography.Text>{cents(data.amount_cents)}</Typography.Text>
            </div>
          ),
          okText: "关闭"
        });
        message.success("订单已创建，请使用微信扫码支付");
        return;
      }
      message.success("订单已创建，请按支付页面提示完成支付");
    },
    onError: (_error, variables) => {
      closePendingPaymentWindow(variables.payWindow);
    }
  });

  useEffect(() => {
    if (!isWeChatBrowser || autoPayHandled) return;
    const wechatPayCode = params.get("wechat_pay_code") || "";
    if (params.get("wechat_pay") !== "1" || !wechatPayCode) return;
    const nextAmount = Number(params.get("amount_yuan"));
    const normalizedAmount = amountOptions.includes(nextAmount) ? nextAmount : amountYuan;
    setAutoPayHandled(true);
    setAmountYuan(normalizedAmount);
    canonicalizeWeChatRechargeURL();
    mutation.mutate({ nextAmountYuan: normalizedAmount, nextPaymentMethod: "wechat", wechatPayCode });
  }, [amountYuan, autoPayHandled, isWeChatBrowser, mutation, params]);

  const handlePay = () => {
    if (paymentMethod === "wechat" && isWeChatBrowser) {
      startWeChatPayAuth(`${WECHAT_RECHARGE_PATH}?wechat_pay=1&amount_yuan=${amountYuan}`);
      return;
    }
    if (paymentMethod === "alipay" && isWeChatBrowser && !alipayWeChatNoticeShown) {
      showWeChatAlipayNotice();
      return;
    }
    const payWindow = paymentMethod === "alipay" && !isWeChatBrowser ? openPendingPaymentWindow() : undefined;
    mutation.mutate({ nextAmountYuan: amountYuan, nextPaymentMethod: paymentMethod, payWindow });
  };
  const handlePaymentMethodChange = (method: PaymentMethod) => {
    setPaymentMethod(method);
    if (method === "alipay" && isWeChatBrowser && !alipayWeChatNoticeShown) {
      showWeChatAlipayNotice();
    }
  };
  const showWeChatAlipayNotice = () => {
    setAlipayWeChatNoticeShown(true);
    modal.info({
      title: "微信内使用支付宝提示",
      content: WECHAT_ALIPAY_NOTICE,
      okText: "我知道了"
    });
  };
  const paidOrders = useMemo(() => (orders.data?.items || []).filter((item) => item.status === "paid"), [orders.data?.items]);

  return (
    <div className="page-stack">
      <div className="page-title-row">
        <h1 className="page-title">充值</h1>
      </div>
      <div className="center-module-grid">
        <Card title="选择充值金额" className="recharge-card">
          <div className="recharge-panel">
            <Radio.Group
              className="amount-grid"
              value={amountYuan}
              onChange={(event) => setAmountYuan(event.target.value)}
              optionType="button"
              buttonStyle="solid"
            >
              {amountOptions.map((amount) => (
                <Radio.Button key={amount} value={amount}>
                  ¥{amount}
                </Radio.Button>
              ))}
            </Radio.Group>
            <div className="three-grid recharge-summary-grid">
              <Card size="small">
                <Statistic title="支付金额" value={amountYuan} prefix="¥" precision={2} />
              </Card>
              <Card size="small">
                <Statistic title="到账点数" value={amountYuan * 100} formatter={(value) => points(Number(value))} />
              </Card>
            </div>
            <div className="payment-method-grid" role="radiogroup" aria-label="选择支付方式">
              <button
                type="button"
                className={`payment-method-card ${paymentMethod === "wechat" ? "active" : ""}`}
                onClick={() => handlePaymentMethodChange("wechat")}
                aria-checked={paymentMethod === "wechat"}
                role="radio"
              >
                <WeChatPayIcon />
                <span>
                  <strong>微信支付</strong>
                  <small>{isWeChatBrowser ? "微信内直接调起支付" : "扫码完成支付"}</small>
                </span>
              </button>
              <button
                type="button"
                className={`payment-method-card alipay-method ${paymentMethod === "alipay" ? "active" : ""}`}
                onClick={() => handlePaymentMethodChange("alipay")}
                aria-checked={paymentMethod === "alipay"}
                role="radio"
              >
                <AlipayIcon />
                <span>
                  <strong>支付宝</strong>
                  <small>{isMobileBrowser ? "跳转支付宝完成支付" : "打开支付宝收银台"}</small>
                </span>
              </button>
            </div>
            <Alert type="info" showIcon message="点数充值后不可退款、不可转赠，请确认金额后再支付。" />
            <Button type="primary" size="large" loading={mutation.isPending} icon={<CreditCard size={16} />} onClick={handlePay}>
              {paymentMethod === "alipay" ? "支付宝支付" : "微信支付"}
            </Button>
          </div>
        </Card>
        <Card title="充值记录" className="record-list-card">
          {orders.isPending ? (
            <div className="record-list-placeholder">加载中...</div>
          ) : paidOrders.length ? (
            <div className="record-list">
              {paidOrders.slice(0, 12).map((item) => (
                <div className="record-row" key={item.id}>
                  <div className="record-main">
                    <div className="record-title">{paymentProviderLabel(item.provider)}</div>
                    <div className="record-meta">支付时间：{time(item.paid_at || item.created_at)}</div>
                  </div>
                  <div className="record-side">
                    <div className="record-amount record-positive">+{points(item.coins)}</div>
                    <div className="record-sub">{cents(item.amount_cents)}</div>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无充值记录" />
          )}
        </Card>
      </div>
    </div>
  );
}

function paymentChannel(method: PaymentMethod, isWeChatBrowser: boolean, isMobileBrowser: boolean) {
  if (method === "alipay") return isMobileBrowser ? "alipay_wap" : "alipay_page";
  return isWeChatBrowser ? "wechat_jsapi" : "wechat_native";
}

function paymentProviderLabel(provider: string) {
  if (provider === "alipay") return "支付宝";
  return "微信支付";
}

function openPendingPaymentWindow() {
  const payWindow = window.open("", "_blank");
  if (!payWindow) {
    return null;
  }
  try {
    payWindow.opener = null;
    payWindow.document.write(`
<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>正在打开支付宝支付</title>
    <style>
      body {
        margin: 0;
        min-height: 100vh;
        display: grid;
        place-items: center;
        font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
        color: #1f2937;
        background: #f8fafc;
      }
      main {
        width: min(360px, calc(100vw - 48px));
        text-align: center;
        line-height: 1.7;
      }
      strong {
        display: block;
        margin-bottom: 8px;
        font-size: 18px;
      }
      span {
        color: #64748b;
      }
    </style>
  </head>
  <body>
    <main>
      <strong>正在打开支付宝支付</strong>
      <span>请稍候，不要关闭此页面。</span>
    </main>
  </body>
</html>`);
    payWindow.document.close();
  } catch {
    // Some embedded browsers disallow writing to the new page; navigation can still work.
  }
  return payWindow;
}

function openPaymentURL(payURL: string, payWindow?: Window | null) {
  if (!payWindow || payWindow.closed) {
    return false;
  }
  try {
    payWindow.location.href = payURL;
    return true;
  } catch {
    return false;
  }
}

function closePendingPaymentWindow(payWindow?: Window | null) {
  if (!payWindow || payWindow.closed) {
    return;
  }
  try {
    payWindow.close();
  } catch {
    // Ignore embedded browser close restrictions.
  }
}

function isWeChatJSPayment(value: unknown): value is WeChatJSPayment {
  const payment = value as Partial<WeChatJSPayment> | undefined;
  return (
    payment?.channel === "wechat_jsapi" &&
    typeof payment.appId === "string" &&
    typeof payment.timeStamp === "string" &&
    typeof payment.nonceStr === "string" &&
    typeof payment.package === "string" &&
    typeof payment.signType === "string" &&
    typeof payment.paySign === "string"
  );
}

function canonicalizeWeChatRechargeURL() {
  if (window.location.pathname !== WECHAT_RECHARGE_PATH || window.location.search) {
    window.history.replaceState(null, "", WECHAT_RECHARGE_PATH);
  }
}

function invokeWeChatPay(payment: WeChatJSPayment) {
  return new Promise<void>((resolve, reject) => {
    let settled = false;
    let timer: number | undefined;
    const cleanup = () => {
      if (timer) {
        window.clearTimeout(timer);
        timer = undefined;
      }
      document.removeEventListener("WeixinJSBridgeReady", invoke);
    };
    const finish = (action: () => void) => {
      if (settled) return;
      settled = true;
      cleanup();
      action();
    };
    const invoke = () => {
      if (settled) return;
      if (!window.WeixinJSBridge) {
        return;
      }
      cleanup();
      const { channel: _channel, ...payRequest } = payment;
      window.WeixinJSBridge.invoke("getBrandWCPayRequest", payRequest, (result) => {
        const message = result.err_msg || "";
        if (message.includes(":ok")) {
          finish(resolve);
          return;
        }
        if (message.includes(":cancel")) {
          finish(() => reject(new Error("已取消微信支付")));
          return;
        }
        finish(() => reject(new Error("微信支付未完成，请重试")));
      });
    };
    if (window.WeixinJSBridge) {
      invoke();
      return;
    }
    document.addEventListener("WeixinJSBridgeReady", invoke, { once: true });
    timer = window.setTimeout(() => finish(() => reject(new Error("微信支付组件未就绪，请稍后重试"))), 30000);
  });
}

function WeChatPayIcon() {
  return (
    <span className="pay-brand-icon wechat-pay-icon" aria-hidden="true">
      <img src="/pay-wechat-icon.svg" alt="" />
    </span>
  );
}

function AlipayIcon() {
  return (
    <span className="pay-brand-icon alipay-icon" aria-hidden="true">
      <img src="/pay-alipay-icon.png" alt="" />
    </span>
  );
}
