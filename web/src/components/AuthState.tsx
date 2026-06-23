import { useQuery } from "@tanstack/react-query";
import { Result, Spin } from "antd";
import type { ReactNode } from "react";
import { Navigate, useLocation } from "react-router-dom";
import { api } from "../api/client";

export function AuthState({ adminOnly, children }: { adminOnly?: boolean; children: ReactNode }) {
  const me = useQuery({ queryKey: ["me"], queryFn: api.me, retry: false });
  const location = useLocation();

  if (me.isPending) {
    return (
      <div className="center-state">
        <Spin />
      </div>
    );
  }

  if (me.isError) {
    const returnTo = `${location.pathname}${location.search}`;
    return <Navigate to={`/login?return_to=${encodeURIComponent(returnTo)}`} replace />;
  }

  if (adminOnly && !me.data.is_admin) {
    return <Result status="403" title="无权访问" subTitle="当前账号不是管理员，无法访问管理后台。" />;
  }

  return <>{children}</>;
}
