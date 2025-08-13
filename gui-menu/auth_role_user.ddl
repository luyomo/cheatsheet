CREATE TABLE `auth_role_user` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `role_id` int(11) NOT NULL,
  `user_name` varchar(128) NOT NULL,
  PRIMARY KEY (`id`)
);
