import { Card, Empty, Spin } from "antd";
import type { ReactNode } from "react";

type DataCardProps = {
  title: ReactNode;
  extra?: ReactNode;
  loading?: boolean;
  empty?: boolean;
  children: ReactNode;
  className?: string;
};

export function DataCard({ title, extra, loading, empty, children, className }: DataCardProps) {
  return (
    <Card title={title} extra={extra} className={className}>
      <Spin spinning={!!loading}>{empty ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} /> : children}</Spin>
    </Card>
  );
}
