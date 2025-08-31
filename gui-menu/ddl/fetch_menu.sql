
with recursive
base_table as (
  select t1.app, t3.id, parent_menu_id 
        from auth_roles t1
  inner join auth_role_menu t2
          on t1.id = t2.role_id
         and t1.role_name = 'default'
	 and t1.app       = '%s'
	 and t1.app       = t2.app
  inner join menu_master t3
          on t2.menu_id = t3.id
	 and t1.app     = t3.app
  union
  select t1.app, t1.id, parent_menu_id 
    from menu_master t1, auth_role_user t2 
   where t2.menu_id = t1.id 
     and t2.user_name = '%s'
     and t1.app       = '%s'
     and t1.app       = t2.app
),
-- AAll the children of the base_table should hbe returned
tmp_table01 as (
  select app, id, parent_menu_id from base_table
  union all
  select t2.app, t1.id, t1.parent_menu_id 
    from menu_master t1, tmp_table01 t2 
   where t2.parent_menu_id = t1.id
     and t1.app            = t2.app
),
-- All the oarent of the base_table should be returned
tmp_table02 as (
  select app, id, parent_menu_id from base_table
  union all
  select t1.app, t1.id, t1.parent_menu_id 
    from menu_master t1, tmp_table02 t2 
   where t1.parent_menu_id = t2.id
     and t1.app            = t2.app
)
select t1.id, t1.parent_menu_id, t1.path, t1.name, coalesce(t1.component, '') as component, coalesce(t1.component_params, '') as component_params
 from menu_master t1 inner join (
  select * from tmp_table01
  union
  select * from tmp_table02
) t2 on t1.id = t2.id
order by t1.parent_menu_id, t1.sort_id, t1.id;
