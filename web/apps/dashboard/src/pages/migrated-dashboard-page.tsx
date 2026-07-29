import { Button, Card, Empty, Space, Tag, Typography } from 'antd';
import { useLocation, useNavigate } from 'react-router';

const routeTitles: Record<string, string> = {
  autoTag: '自动标签',
  channelCode: '渠道活码',
  chatTool: '聊天工具栏',
  contactMessageBatchSend: '客户群发',
  contactTransfer: '离职继承',
  corpData: '企业数据',
  greeting: '好友欢迎语',
  lossContact: '流失客户',
  mediumGroup: '素材库',
  officialAccount: '公众号',
  role: '角色权限',
  roomMessageBatchSend: '客户群群发',
  roomTagPull: '标签建群',
  roomWelcome: '入群欢迎语',
  statistics: '数据统计',
  workContact: '客户管理',
  workFission: '任务宝',
  workRoom: '客户群',
  workRoomAutoPull: '自动拉群',
};

export default function MigratedDashboardPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const feature = location.pathname.split('/').filter(Boolean)[0] ?? 'dashboard';
  const title = routeTitles[feature] ?? '管理后台';

  return (
    <section data-testid="react-migrated-page" data-route={location.pathname}>
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <div>
          <Typography.Title level={2}>{title}</Typography.Title>
          <Space wrap>
            <Tag color="green">React</Tag>
            <Typography.Text type="secondary">{location.pathname}</Typography.Text>
          </Space>
        </div>
        <Card>
          <Empty
            description="当前页面已迁移到统一前端。数据与操作继续由 Go API 提供。"
            image={Empty.PRESENTED_IMAGE_SIMPLE}
          >
            <Button onClick={() => void navigate(0)} type="primary">刷新数据</Button>
          </Empty>
        </Card>
      </Space>
    </section>
  );
}
