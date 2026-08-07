UPDATE mc_rbac_role SET remarks = 'bootstrap full-access role', updated_at = NOW()
WHERE remarks = '系统预置全权限角色';
