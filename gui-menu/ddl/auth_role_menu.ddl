CREATE TABLE `auth_role_menu` (
  `app` varchar(64) NOT NULL,
  `component` varchar(64) NOT NULL,
  `id` bigint(20) NOT NULL,
  `role_id` int(11) DEFAULT NULL,
  `menu_id` int(11) DEFAULT NULL,
  PRIMARY KEY (`app`, `component`, `id`)
);
