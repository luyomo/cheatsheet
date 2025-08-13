CREATE TABLE `menu_master` (
  `sequence` int(11) NOT NULL AUTO_INCREMENT,
  `parent_menu_id` int(11) NOT NULL,
  `path` varchar(64) NOT NULL,
  `name` varchar(64) NOT NULL,
  `component` varchar(64) DEFAULT NULL,
  `component_params` longtext CHARACTER SET utf8mb4 COLLATE utf8mb4_bin DEFAULT NULL CHECK (json_valid(`component_params`)),
  `sort_id` int(11) DEFAULT NULL,
  PRIMARY KEY (`sequence`)
) ENGINE=InnoDB AUTO_INCREMENT=9104 DEFAULT CHARSET=utf8mb4;
