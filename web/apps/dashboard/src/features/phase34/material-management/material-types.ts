export type MaterialScope = 'public' | 'department' | 'personal' | 'sidebar';

export type MaterialRecord = {
  id: number;
  type: string;
  content: Record<string, unknown>;
  mediumGroupId: number;
  mediumGroupName: string;
  scopeType: string;
  status: string;
  sidebarVisible: boolean;
  userName: string;
  createdAt: string;
};

export type MaterialGroup = { id: number; name: string };

export function materialTitle(item: MaterialRecord): string {
  const value = item.content.title ?? item.content.name ?? item.content.fileName;
  return typeof value === 'string' && value.trim() ? value : `素材 #${item.id}`;
}

export function materialPreview(item: MaterialRecord): string {
  const value = item.content.content ?? item.content.description ?? item.content.imageFullPath ?? item.content.fileFullPath;
  return typeof value === 'string' && value.trim() ? value : materialTitle(item);
}
