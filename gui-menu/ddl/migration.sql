insert into webmasterdb.menu_master select app, 'main', id, parent_menu_id, path, name, component, component_params, sort_id from menu_master; 
insert into webmasterdb.auth_roles  select app, 'main', id, role_name from auth_roles;
insert into webmasterdb.auth_role_user select app, 'main',  id, role_id, user_name from auth_role_user;
insert into webmasterdb.auth_role_menu select app, 'main', id, role_id, menu_id from auth_role_menu;
