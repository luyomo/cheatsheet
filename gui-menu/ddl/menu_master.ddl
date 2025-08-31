CREATE TABLE `menu_master` (
  `app` varchar(64) NOT NULL,
  `module` varchar(64) NOT NULL,
  `id` int(11) NOT NULL,
  `parent_menu_id` int(11) NOT NULL,
  `path` varchar(64) NOT NULL,
  `name` varchar(64) NOT NULL,
  `component` varchar(64) DEFAULT NULL,
  `component_params` longtext CHARACTER SET utf8mb4 COLLATE utf8mb4_bin DEFAULT NULL,
  `sort_id` int(11) DEFAULT NULL,
  PRIMARY KEY (`app`, `module`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
