DELETE FROM `mc_rbac_menu`
WHERE `id` IN (459, 460, 461, 462, 463, 464, 465, 466, 467, 468, 469, 532, 533, 534, 535, 536, 537, 538)
  AND `link_url` IN (
    '/dashboard/contactSop/index#get',
    '/dashboard/contactSop/delete#delete',
    '/dashboard/contactSop/destroy#delete',
    '/dashboard/contactSop/detail#get',
    '/dashboard/contactSop/edit#put',
    '/dashboard/contactSop/info#get',
    '/dashboard/contactSop/logState#put',
    '/dashboard/contactSop/setEmployee#put',
    '/dashboard/contactSop/state#put',
    '/dashboard/contactSop/store#post',
    '/dashboard/contactSop/update#put',
    '/dashboard/roomSop/index#get',
    '/dashboard/roomSop/destroy#delete',
    '/dashboard/roomSop/info#get',
    '/dashboard/roomSop/setRoom#put',
    '/dashboard/roomSop/state#put',
    '/dashboard/roomSop/store#post',
    '/dashboard/roomSop/update#put'
  );
