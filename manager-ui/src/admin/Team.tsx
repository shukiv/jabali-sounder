import { useTranslation } from "react-i18next";
import { useState } from "react";
import {
  Card,
  Table,
  Button,
  Modal,
  Form,
  Input,
  Select,
  Tag,
  Space,
  Popconfirm,
  App,
} from "antd";
import { PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import {
  useAdmins,
  useCreateAdmin,
  useUpdateAdmin,
  useDeleteAdmin,
} from "../hooks/useAdmins";
import type { Admin } from "../types";
import { sortable } from "../lib/tableSort";

const ROLE_OPTIONS = [
  { label: "Viewer (read-only)", value: "viewer" },
  { label: "Operator (manage servers)", value: "operator" },
  { label: "Owner (manage operators)", value: "owner" },
];

const ROLE_COLOR: Record<string, string> = {
  viewer: "default",
  operator: "blue",
  owner: "gold",
};

// Team is the operator-management page (M3: RBAC). Owner-only — the API enforces
// it and the nav entry only shows for owners.
export default function Team() {
  const { t } = useTranslation();
  const { data: admins, isLoading } = useAdmins();
  const createMut = useCreateAdmin();
  const updateMut = useUpdateAdmin();
  const deleteMut = useDeleteAdmin();
  const { message } = App.useApp();
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm();

  const handleCreate = async () => {
    try {
      const values = await form.validateFields();
      await createMut.mutateAsync(values);
      message.success(`Operator "${values.username}" created`);
      setOpen(false);
      form.resetFields();
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const changeRole = async (id: string, role: string) => {
    try {
      await updateMut.mutateAsync({ id, role });
      message.success(t("team.role_updated"));
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const handleDelete = async (id: string, username: string) => {
    try {
      await deleteMut.mutateAsync(id);
      message.success(`Removed "${username}"`);
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const columns = [
    { title: t("team.username"), dataIndex: "username", key: "username" },
    {
      title: t("team.role"),
      dataIndex: "role",
      key: "role",
      render: (role: string, record: Admin) => (
        <Select
          size="small"
          value={role}
          options={ROLE_OPTIONS}
          style={{ minWidth: 220 }}
          aria-label={`Change role for ${record.username}`}
          onChange={(v) => changeRole(record.id, v)}
        />
      ),
    },
    {
      title: t("team.current"),
      key: "tag",
      render: (_: unknown, record: Admin) => (
        <Tag color={ROLE_COLOR[record.role]}>{record.role}</Tag>
      ),
    },
    {
      title: t("team.actions"),
      key: "actions",
      render: (_: unknown, record: Admin) => (
        <Popconfirm
          title={`Remove "${record.username}"?`}
          okText={t("team.remove")}
          okButtonProps={{ danger: true }}
          onConfirm={() => handleDelete(record.id, record.username)}
        >
          <Button danger size="small" icon={<DeleteOutlined />}>
            Remove
          </Button>
        </Popconfirm>
      ),
    },
  ];

  return (
    <div>
      <Space wrap style={{ marginBottom: 16, width: "100%", justifyContent: "space-between" }}>
        <h3 style={{ margin: 0 }}>Team</h3>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>
          Add operator
        </Button>
      </Space>
      <Card>
        <Table<Admin>
          dataSource={admins || []}
          columns={sortable(columns)}
          rowKey="id"
          loading={isLoading}
          pagination={false}
          scroll={{ x: "max-content" }}
        />
      </Card>

      <Modal
        title={t("team.add_operator")}
        open={open}
        onOk={handleCreate}
        confirmLoading={createMut.isPending}
        onCancel={() => {
          setOpen(false);
          form.resetFields();
        }}
        okText={t("team.create")}
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="username"
            label={t("team.username")}
            rules={[{ required: true, message: t("team.username_is_required") }]}
          >
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="password"
            label={t("team.password")}
            rules={[{ required: true, min: 8, message: t("team.at_least_8_characters") }]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Form.Item
            name="role"
            label={t("team.role")}
            initialValue="viewer"
            rules={[{ required: true }]}
          >
            <Select options={ROLE_OPTIONS} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
