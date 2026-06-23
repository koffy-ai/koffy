import { useQuery } from "@tanstack/react-query";
import { Button, Empty, Input, List, Select, Space, Tag, Typography } from "antd";
import { Search } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { api } from "../../api/client";
import type { AdminUserSearchItem } from "../../api/types";

type SelectOption = {
  value: string;
  label?: string;
};

export function AdminSelect({
  value,
  onChange,
  options,
  placeholder,
  width = "100%",
  allowClear = true,
  disabled
}: {
  value?: string;
  onChange?: (value?: string) => void;
  options: SelectOption[];
  placeholder?: string;
  width?: number | string;
  allowClear?: boolean;
  disabled?: boolean;
}) {
  return (
    <Select
      allowClear={allowClear}
      disabled={disabled}
      placeholder={placeholder}
      value={value || undefined}
      onChange={onChange}
      style={{ width }}
      options={options.map((item) => ({
        value: item.value,
        label: item.label ? `${item.label} (${item.value})` : item.value
      }))}
      notFoundContent={<Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无数据" />}
    />
  );
}

export function AdminUserSearchSelect({
  value,
  onChange,
  width = 320,
  placeholder = "输入手机号、昵称或用户 ID"
}: {
  value?: string;
  onChange?: (value?: string) => void;
  width?: number | string;
  placeholder?: string;
}) {
  const [keyword, setKeyword] = useState(value || "");
  const [query, setQuery] = useState("");
  const [selectedUser, setSelectedUser] = useState<AdminUserSearchItem>();
  const users = useQuery({
    queryKey: ["admin-user-search", query],
    queryFn: () => api.adminSearchUsers(query, 20),
    enabled: !!query
  });

  useEffect(() => {
    setKeyword(value || "");
    if (!value) {
      setSelectedUser(undefined);
      setQuery("");
    }
  }, [value]);

  const items = users.data?.items || [];
  const selected = useMemo(() => {
    if (selectedUser?.casdoor_user_id === value) {
      return selectedUser;
    }
    return items.find((item) => item.casdoor_user_id === value);
  }, [items, selectedUser, value]);

  const search = () => {
    const next = keyword.trim();
    setQuery(next);
  };

  const choose = (item: AdminUserSearchItem) => {
    setKeyword(item.casdoor_user_id);
    setSelectedUser(item);
    setQuery("");
    onChange?.(item.casdoor_user_id);
  };

  return (
    <div className="admin-user-search-select" style={{ width }}>
      <Space.Compact style={{ width: "100%" }}>
        <Input
          allowClear
          value={keyword}
          placeholder={placeholder}
          onChange={(event) => {
            setKeyword(event.target.value);
            if (!event.target.value) {
              setQuery("");
              onChange?.(undefined);
            }
          }}
          onPressEnter={search}
        />
        <Button icon={<Search size={16} />} loading={users.isFetching} onClick={search} />
      </Space.Compact>
      {selected ? (
        <Typography.Text className="admin-selected-user" type="secondary">
          已选：{displayUser(selected)}
        </Typography.Text>
      ) : null}
      {query ? (
        <List
          className="admin-user-search-results"
          size="small"
          bordered
          loading={users.isFetching}
          dataSource={items}
          locale={{ emptyText: "暂无匹配用户" }}
          renderItem={(item) => (
            <List.Item className="admin-user-search-result" onClick={() => choose(item)}>
              <div>
                <Typography.Text strong>{displayUser(item)}</Typography.Text>
                <Typography.Text type="secondary" className="admin-user-search-meta">
                  {item.casdoor_user_id}
                </Typography.Text>
              </div>
              {item.is_admin ? <Tag color="blue">管理员</Tag> : null}
            </List.Item>
          )}
        />
      ) : null}
    </div>
  );
}

function displayUser(item: AdminUserSearchItem) {
  return item.display_name || item.name || item.phone || item.casdoor_user_id;
}
