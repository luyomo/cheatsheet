CREATE TABLE `auth_roles` (
  `app` varchar(64) NOT NULL,
  `component` varchar(64) NOT NULL,
  `id` int(11) NOT NULL,
  `role_name` varchar(64) NOT NULL,
  PRIMARY KEY (`app`, `component`, `id`) );
