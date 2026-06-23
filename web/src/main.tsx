import { MutationCache, QueryCache, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App as AntApp, ConfigProvider } from "antd";
import zhCN from "antd/locale/zh_CN";
import React, { useEffect, useState } from "react";
import ReactDOM from "react-dom/client";
import { RouterProvider } from "react-router-dom";
import { isAuthError } from "./api/client";
import { router } from "./router";
import "./styles/global.css";

function AppProviders() {
  const { message } = AntApp.useApp();
  const [queryClient] = useState(
    () =>
      new QueryClient({
        queryCache: new QueryCache({
          onError: (error) => {
            if (!isAuthError(error)) {
              message.error(error.message || "请求失败");
            }
          }
        }),
        mutationCache: new MutationCache({
          onError: (error, _variables, _context, mutation) => {
            const meta = mutation.meta as { skipGlobalError?: boolean } | undefined;
            if (!isAuthError(error) && !meta?.skipGlobalError) {
              message.error(error.message || "提交失败");
            }
          }
        }),
		defaultOptions: {
		  queries: {
			refetchOnWindowFocus: true,
			refetchOnReconnect: true,
			retry: 1
		  }
		}
      })
	  );

  useEffect(() => {
    const syncAuthState = (event: StorageEvent) => {
      if (event.key === "koffy-auth-updated") {
        queryClient.invalidateQueries({ queryKey: ["me"] });
        queryClient.invalidateQueries({ queryKey: ["auth-bindings"] });
      }
    };
    window.addEventListener("storage", syncAuthState);
    return () => window.removeEventListener("storage", syncAuthState);
  }, [queryClient]);

  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ConfigProvider
      locale={zhCN}
      theme={{
        token: {
          colorPrimary: "#1769aa",
          borderRadius: 6,
          fontFamily:
            '-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif'
        },
        components: {
          Card: { borderRadiusLG: 6 },
          Button: { borderRadius: 6 },
          Table: { headerBg: "#f4f7fb" }
        }
      }}
    >
      <AntApp>
        <AppProviders />
      </AntApp>
    </ConfigProvider>
  </React.StrictMode>
);
