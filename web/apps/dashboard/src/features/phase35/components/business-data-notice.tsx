export const businessDataNoticeText = '部分数据暂未同步完整，当前仅展示已获取的数据。';

export function BusinessDataNotice({ visible = true }: { visible?: boolean }) {
  if (!visible) return null;
  return <p className="phase35-business-notice" role="status">{businessDataNoticeText}</p>;
}
