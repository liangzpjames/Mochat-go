DELETE FROM `mc_rbac_role_menu` WHERE `menu_id` BETWEEN 116001 AND 116009;
DELETE FROM `mc_rbac_menu` WHERE `id` BETWEEN 116001 AND 116009;
DROP TABLE IF EXISTS `mc_phase34_short_link_visits`;
DROP TABLE IF EXISTS `mc_phase34_short_links`;
DROP TABLE IF EXISTS `mc_phase34_customer_services`;
DROP TABLE IF EXISTS `mc_phase34_acquisition_links`;
