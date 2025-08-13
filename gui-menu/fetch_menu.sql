with recursive
base_table as (
  select t3.sequence, parent_menu_id from auth_roles t1
  inner join auth_role_menu t2
          on t1.id = t2.role_id
         and t1.role_name = 'default'
  inner join menu_master t3
          on t2.menu_id = t3.sequence
  union
  select sequence, parent_menu_id from menu_master t1, v_user_menu t2 where t2.menu_id = t1.sequence and t2.user_name = '%s'
),
tmp_table01 as (
  select sequence, parent_menu_id from base_table
  union all
  select t1.sequence, t1.parent_menu_id from menu_master t1, tmp_table01 t2 where t2.parent_menu_id = t1.sequence
),
tmp_table02 as (
  select sequence, parent_menu_id from base_table
  union all
  select t1.sequence, t1.parent_menu_id from menu_master t1, tmp_table02 t2 where t1.parent_menu_id = t2.sequence
)
select t1.sequence, t1.parent_menu_id, t1.path, t1.name, coalesce(t1.component, '') as component, coalesce(t1.component_params, '') as component_params
 from menu_master t1 inner join (
  select * from tmp_table01
  union
  select * from tmp_table02
) t2 on t1.sequence = t2.sequence
order by t1.parent_menu_id, t1.sort_id, t1.sequence
