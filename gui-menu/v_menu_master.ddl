CREATE VIEW v_menu_master 
AS 
  select sequence
, parent_menu_id
, path
, name
, component
, sort_id
from menu_master;
