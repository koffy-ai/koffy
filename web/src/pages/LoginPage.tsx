import { useQuery } from "@tanstack/react-query";
import { App } from "antd";
import { useCallback, useEffect, useMemo, useRef } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { api } from "../api/client";
import { LoginBox } from "../components/LoginBox";
import { setBrandingFavicon } from "../components/branding";

export function LoginPage() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const returnTo = useMemo(() => safeReturnTo(params.get("return_to")), [params]);
  const loginCode = params.get("koffy_login_code") || "";
  const loginCodeExchanged = useRef(false);
  const me = useQuery({ queryKey: ["me"], queryFn: api.me, retry: false, refetchOnWindowFocus: true });
  const { refetch } = me;
  const redirectIfLoggedIn = useCallback(() => {
    if (document.hidden) return;
    refetch()
      .then((result) => {
        if (result.data) {
          navigate(returnTo, { replace: true });
        }
      })
      .catch(() => {
        // Still not logged in.
      });
  }, [navigate, refetch, returnTo]);

  useEffect(() => {
    if (!loginCode || loginCodeExchanged.current) return;
    loginCodeExchanged.current = true;
    api
      .exchangeSession({ code: loginCode })
      .then(() => {
        localStorage.setItem("koffy-auth-updated", String(Date.now()));
        navigate(returnTo, { replace: true });
      })
      .catch((error) => {
        message.error(error instanceof Error ? error.message : "微信登录状态已失效，请重新扫码");
        const next = new URLSearchParams(params);
        next.delete("koffy_login_code");
        setParams(next, { replace: true });
      });
  }, [loginCode, message, navigate, params, returnTo, setParams]);

  useEffect(() => {
    const errorMessage = params.get("wechat_error");
    if (!errorMessage) return;
    message.error(errorMessage);
    const next = new URLSearchParams(params);
    next.delete("wechat_error");
    next.delete("wechat_error_code");
    setParams(next, { replace: true });
  }, [message, params, setParams]);

  useEffect(() => {
    if (me.isSuccess) {
      navigate(returnTo, { replace: true });
    }
  }, [me.isSuccess, navigate, returnTo]);

  useEffect(() => {
    document.title = returnTo.startsWith("/admin") ? "管理后台" : "用户中心";
    setBrandingFavicon(returnTo.startsWith("/admin") ? "admin" : "center");
  }, [returnTo]);

  useEffect(() => {
    const syncLoginState = (event: StorageEvent) => {
      if (event.key === "koffy-auth-updated") {
        redirectIfLoggedIn();
      }
    };

    window.addEventListener("focus", redirectIfLoggedIn);
    window.addEventListener("pageshow", redirectIfLoggedIn);
    document.addEventListener("visibilitychange", redirectIfLoggedIn);
    window.addEventListener("storage", syncLoginState);
    return () => {
      window.removeEventListener("focus", redirectIfLoggedIn);
      window.removeEventListener("pageshow", redirectIfLoggedIn);
      document.removeEventListener("visibilitychange", redirectIfLoggedIn);
      window.removeEventListener("storage", syncLoginState);
    };
  }, [redirectIfLoggedIn]);

  return (
    <div className="auth-page">
      <div className="auth-shell auth-shell-simple">
        <LoginBox returnTo={returnTo} />
      </div>
    </div>
  );
}

function safeReturnTo(value: string | null) {
  if (!value) return "/center";
  try {
    const target = new URL(value, window.location.origin);
    if (target.origin !== window.location.origin) return "/center";
    const path = `${target.pathname}${target.search}${target.hash}`;
    if (target.pathname === "/login" || target.pathname === "/register" || target.pathname === "/forgot-password") {
      return "/center";
    }
    return path || "/center";
  } catch {
    return "/center";
  }
}
