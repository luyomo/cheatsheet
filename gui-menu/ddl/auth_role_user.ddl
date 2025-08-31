CREATE TABLE `auth_role_user` (
  `app` varchar(64) NOT NULL,
  `component` varchar(64) NOT NULL,
  `id` int(11) NOT NULL,
  `role_id` int(11) NOT NULL,
  `user_name` varchar(128) NOT NULL,
  PRIMARY KEY (`app`, `component`, `id`)
);
