DELETE FROM `mc_rbac_menu`
WHERE `id` IN (414, 415, 416, 417, 418, 419, 420, 421, 422, 423, 424, 436, 437, 438, 439)
  AND `link_url` IN (
    '/dashboard/contactBatchAdd/index',
    '/dashboard/contactBatchAdd/index#get',
    '/dashboard/contactBatchAdd/importIndex#get',
    '/dashboard/contactBatchAdd/importStore#post',
    '/dashboard/contactBatchAdd/remind#get',
    '/dashboard/contactBatchAdd/settingEdit#get',
    '/dashboard/contactBatchAdd/settingUpdate#post',
    '/dashboard/contactBatchAdd/importDestroy#delete',
    '/dashboard/contactBatchAdd/destroy#delete',
    '/dashboard/contactBatchAdd/allot#post',
    '/dashboard/contactBatchAdd/dataStatistic#get',
    '/contactBatchAdd/importIndex',
    '/contactBatchAdd/importShow',
    '/contactBatchAdd/dataStatistic',
    '/contactBatchAdd/dataShow'
  );
