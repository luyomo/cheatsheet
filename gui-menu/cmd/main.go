package main

import (
    "github.com/gin-gonic/gin"
    "gopkg.in/yaml.v2"
    "encoding/json"

    "fmt"
    "os"
    "net/http"

    db "github.com/luyomo/cheatsheet/gui-menu/pkg/database"
)

// Config struct to hold configuration
type Config struct {
    Database struct {
        Host     string `yaml:"host"`
        Port     int    `yaml:"port"` 
        User     string `yaml:"user"`
        Password string `yaml:"password"`
        DBName   string `yaml:"dbname"`
		UseTLS   bool   `yaml:"usetls""`
    } `yaml:"database"`
}

type MenuRow struct {
	ID              int                    `json:"-"`
	ParentMenuId    int                    `json:"-"`
	Path            string                 `json:"path"`
	Name            string                 `json:"name"`
	Component       string                 `json:"component,omitempty"`
	ComponentParams map[string]interface{} `json:"params,omitempty"`
	Children        []MenuRow              `json:"routes,omitempty"`
}

func main() {
    // Initialize a Gin router (default includes Logger and Recovery middleware)
    r := gin.Default()

    // Load config from yaml file
    config, err := loadConfig("config/config.yaml")
    if err != nil {
        panic(fmt.Errorf("failed to load config: %v", err))
    }
    // fmt.Printf("Config: %#v\n", config)

    db := db.Database{
		//  db, err := sql.Open("mysql", "yomoenuser:yomoenuser@tcp(192.168.1.105:3306)/yomoen")
		Host:     config.Database.Host,
		Port:     config.Database.Port,
		User:     config.Database.User,
		Password: config.Database.Password,
		DBName:   config.Database.DBName,
	}
	err = db.Init(config.Database.UseTLS)
	if err != nil {
		panic(err)
	}
    // fmt.Printf("DB: %#v\n", db)

    r.GET("/api/master/v1/menu", func(c *gin.Context) {
        user   := c.Query("user")
        app    := c.Query("app" )
        module := c.Query("module")

        if module == "" {
            module = "main"
        }

        ret, err := fetchMenuItems(db, app, module, user)
        if err != nil {
            fmt.Printf("Error: %#v\n", err)
            c.JSON(500, gin.H{
                "message": "Internal Server Error",
            })
        } else {
            c.String(http.StatusOK, string(ret))
        }
    })

    // Run the server on port 8080
    r.Run("0.0.0.0:8082") // Listen on 0.0.0.0:8080
}

func loadConfig(path string) (*Config, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    var cfg Config
    decoder := yaml.NewDecoder(f)
    err = decoder.Decode(&cfg)
    if err != nil {
        return nil, err
    }

    return &cfg, nil
}

func fetchMenuItems(db db.Database, app, module, user string) ([]byte , error) {
	var arrData []MenuRow

	// Please refer to the doc for reference. https://github.com/luyomo/yomo-quiz/blob/master/doc/menu_access_by_user.org
	query := fmt.Sprintf(`
    with recursive
    base_table as (
      select t1.app, t1.module, t3.id, parent_menu_id
            from auth_roles t1
      inner join auth_role_menu t2
              on t1.id = t2.role_id
             and t1.role_name = 'default'
             and t1.app       = '%s'
             and t1.app       = t2.app
             and t1.module    = '%s'
             and t1.module    = t2.module
      inner join menu_master t3
              on t2.menu_id = t3.id
             and t1.app     = t3.app
             and t1.module  = t3.module
      union
      select distinct t1.app, t1.module, t1.id, parent_menu_id
        from menu_master t1, auth_role_user t2, auth_role_menu t3
       where t3.menu_id   = t1.id
         and t2.role_id   = t3.role_id
         and t1.app       = '%s'
         and t1.module    = '%s'
         and t2.user_name = '%s'
         and t1.app       = t2.app
         and t2.app       = t3.app
         and t1.module    = t2.module
         and t2.module    = t3.module
    ),
    tmp_table01 as (
      select app, module, id, parent_menu_id from base_table
      union all
      select t2.app, t2.module, t1.id, t1.parent_menu_id
        from menu_master t1, tmp_table01 t2
       where t2.parent_menu_id = t1.id
         and t1.app            = t2.app
         and t1.module         = t2.module
    ),
    tmp_table02 as (
      select app, module, id, parent_menu_id from base_table
      union all
      select t1.app, t1.module, t1.id, t1.parent_menu_id
        from menu_master t1, tmp_table02 t2
       where t1.parent_menu_id = t2.id
         and t1.app            = t2.app
         and t1.module         = t2.module
    )
    select t1.id, t1.parent_menu_id, t1.path, t1.name, coalesce(t1.component, '') as component, coalesce(t1.component_params, '') as component_params
     from menu_master t1 inner join (
      select * from tmp_table01
      union
      select * from tmp_table02
    ) t2 on t1.id = t2.id
        and t1.app = t2.app
        and t1.module = t2.module
    order by t1.parent_menu_id, t1.sort_id, t1.id
    `, app, module, app, module, user)

    // fmt.Println(query)
    rows, err := db.Connection.Query(query)

    if err != nil {
        return nil, err
    }
	for rows.Next() {
		var row MenuRow
		var componentParams string

		if err = rows.Scan(&row.ID, &row.ParentMenuId, &row.Path, &row.Name, &row.Component, &componentParams); err!= nil {
            return nil, err
        }            

		// fmt.Printf("The row: %#v \n", row)

		// fmt.Printf("before parsed component params : %#v \n", componentParams)
		json.Unmarshal([]byte(componentParams), &row.ComponentParams)
		// fmt.Printf("After parsed data: %#v \n", row.ComponentParams)

		pushedFlg := pushMenuRow(row, &arrData)
		if pushedFlg == false {
			row.Path = fmt.Sprintf("/%s", row.Path)
			arrData = append(arrData, row)
		}
		// fmt.Printf("after pushed data: %#v \n\n", arrData)
	}

	// fmt.Printf("Final result: %#v \n", arrData)
	bytesData, err := json.Marshal(arrData)
	if err != nil {
        return nil, err
	}

	// db.Close()
	return bytesData, nil
}

func pushMenuRow(menuRow MenuRow, arrData *[]MenuRow) bool {
	//  for _, row := range *arrData {
	for idx := 0; idx < len((*arrData)); idx++ {
		// fmt.Printf(" ---- %d -> %d \n", menuRow.ParentMenuId, (*arrData)[idx].ID)
		if menuRow.ParentMenuId == (*arrData)[idx].ID {
			menuRow.Path = fmt.Sprintf("%s/%s", (*arrData)[idx].Path, menuRow.Path)
			(*arrData)[idx].Children = append((*arrData)[idx].Children, menuRow)
			return true
		} else {
			ret := pushMenuRow(menuRow, &((*arrData)[idx].Children))
			if ret == true {
				return true
			}
		}
	}

	return false
}
