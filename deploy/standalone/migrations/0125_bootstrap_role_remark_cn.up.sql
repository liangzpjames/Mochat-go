-- 将历史 bootstrap 预置角色的英文备注改为中文，避免直接暴露给业务用户。
UPDATE mc_rbac_role SET remarks = '系统预置全权限角色', updated_at = NOW()
WHERE remarks = 'bootstrap full-access role';
