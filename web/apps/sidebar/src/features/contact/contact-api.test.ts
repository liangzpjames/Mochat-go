import { describe, expect, it, vi } from 'vitest';

import {
  appendContactTags,
  loadContactPortrait,
  loadContactWorkspace,
  loadTagGroups,
  loadTags,
  updateContactPortrait,
  updateContactRemark,
  uploadPortraitImage,
  type ContactPortraitField,
} from './contact-api';

describe('Sidebar contact domain API', () => {
  it('loads the persisted contact workspace by contact id', async () => {
    const request = vi.fn().mockResolvedValue({
      name: '林晓',
      avatar: null,
      gender: 2,
      genderText: '女',
      businessNo: 'C-11',
      remark: '重点客户',
      description: '偏好下午沟通',
      tag: [{ tagId: 7, tagName: '高意向' }],
      roomName: ['客户交流群'],
      employeeName: ['员工甲'],
    });

    await expect(loadContactWorkspace(request, 11)).resolves.toMatchObject({
      remark: '重点客户',
      tags: [{ id: 7, name: '高意向' }],
      roomNames: ['客户交流群'],
    });
    expect(request).toHaveBeenCalledWith('/workContact/show?contactId=11', { method: 'GET' });
  });

  it('rejects malformed workspace data instead of filling fake fields', async () => {
    const request = vi.fn().mockResolvedValue({ name: '林晓', tag: '高意向' });

    await expect(loadContactWorkspace(request, 11)).rejects.toMatchObject({ kind: 'validation' });
  });

  it('loads and normalizes all supported portrait field values', async () => {
    const request = vi.fn().mockResolvedValue([
      { contactFieldId: 31, contactFieldPivotId: 901, name: '爱好', type: 2, typeText: '多选', options: ['跑步', '读书'], value: ['跑步'] },
      { contactFieldId: 32, contactFieldPivotId: '', name: '城市', type: 3, typeText: '单选', options: ['上海'], value: '' },
      { contactFieldId: 33, contactFieldPivotId: 902, name: '照片', type: 11, typeText: '图片', options: [], value: 'portrait/a.png', pictureFlag: '/static/portrait/a.png' },
    ]);

    const fields = await loadContactPortrait(request, 11);

    expect(fields).toHaveLength(3);
    expect(fields[0]?.value).toEqual(['跑步']);
    expect(fields[2]?.pictureUrl).toBe('/static/portrait/a.png');
  });

  it('serializes portrait updates without changing field identity', async () => {
    const request = vi.fn().mockResolvedValue([]);
    const fields: ContactPortraitField[] = [{
      contactFieldId: 31,
      pivotId: 901,
      name: '爱好',
      type: 2,
      typeText: '多选',
      options: ['跑步', '读书'],
      value: ['跑步', '读书'],
      pictureUrl: null,
    }];

    await updateContactPortrait(request, 11, fields);

    expect(request).toHaveBeenCalledWith('/contactFieldPivot/update', expect.objectContaining({
      method: 'PUT',
      body: JSON.stringify({ contactId: 11, userPortrait: fields }),
    }));
  });

  it('writes remarks and appended tags through the real update contract', async () => {
    const request = vi.fn().mockResolvedValue([]);

    await updateContactRemark(request, 11, '新备注');
    await appendContactTags(request, 11, [7, 9, 9]);

    expect(request).toHaveBeenNthCalledWith(1, '/workContact/update', expect.objectContaining({
      method: 'PUT',
      body: JSON.stringify({ contactId: 11, remark: '新备注' }),
    }));
    expect(request).toHaveBeenNthCalledWith(2, '/workContact/update', expect.objectContaining({
      method: 'PUT',
      body: JSON.stringify({ contactId: 11, tag: [7, 9] }),
    }));
  });

  it('loads persisted tag groups and tags with exact query encoding', async () => {
    const request = vi.fn()
      .mockResolvedValueOnce([{ groupId: 3, groupName: '阶段' }])
      .mockResolvedValueOnce([{ id: 7, name: '高意向' }]);

    await expect(loadTagGroups(request)).resolves.toEqual([{ id: 3, name: '阶段' }]);
    await expect(loadTags(request, 3)).resolves.toEqual([{ id: 7, name: '高意向' }]);
    expect(request).toHaveBeenNthCalledWith(2, '/workContactTag/allTag?groupId=3', { method: 'GET' });
  });

  it('uploads a real FormData image and returns its persisted path', async () => {
    const request = vi.fn().mockResolvedValue({ path: 'portrait/new.png', fullPath: '/static/portrait/new.png' });
    const file = new File(['png'], 'portrait.png', { type: 'image/png' });

    await expect(uploadPortraitImage(request, file)).resolves.toEqual({
      path: 'portrait/new.png',
      fullPath: '/static/portrait/new.png',
    });
    const init = request.mock.calls[0]?.[1] as RequestInit;
    expect(request.mock.calls[0]?.[0]).toBe('/common/upload');
    expect(init.method).toBe('POST');
    expect(init.body).toBeInstanceOf(FormData);
    expect(init.headers).toBeUndefined();
  });
});
