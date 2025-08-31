CREATE VIEW v_menu_master 
AS 
  select
  app
, component
, id
, parent_menu_id
, path
, name
, component
, sort_id
from menu_master;
