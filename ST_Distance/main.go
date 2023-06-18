package main

import (
   "fmt"
   "math"
   "math/rand"
   "time"

   "database/sql"
   _ "github.com/go-sql-driver/mysql"
   "github.com/pingcap/tiup/pkg/tui"
   "github.com/spf13/cobra"

   "flag"

)

// https://www.gsi.go.jp/KOKUJYOHO/center.htm
// 136 |<- ->| 154
//  20 |<- ->|  24


// SELECT xxx,st_distance($geo,geo)*111195 as distance FROM `$schema` WHERE  geo is not null and st_distance($geo,geo)*111195 <= :distance order by distance
// create table test.gis_table(id bigint primary key, name varchar(32) not null, position point)

var gOpt Args

func main() {
    // lat1 := 29.490295
    // lng1 := 106.486654

    // lat2 := 29.615467
    // lng2 := 126.581515

        rootCmd := &cobra.Command{
            Use:           tui.OsArgs0(),
            Short:         "How to migrate st_instance to TiDB",
            SilenceUsage:  true,
            SilenceErrors: true,
            Version:       "0.0.1",
            PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
                fmt.Printf("Starting to run PersistentPreRunE \n")
                return nil
            },
            PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
                fmt.Printf("Calling PersistentPostRunE \n")
                    // proxy.MaybeStopProxy()
                    // return tiupmeta.GlobalEnv().V1Repository().Mirror().Close()
                return nil
            },
            Run: func(cmd *cobra.Command, args []string) {
                fmt.Printf("Calling func")
            },
    }
    rootCmd.PersistentFlags().StringVar(&gOpt.dbType, "db-type", "TiDB", "db type(TiDB or MySQL)")
    rootCmd.PersistentFlags().StringVar(&gOpt.dbHost, "host", "127.0.0.1", "db host")
    rootCmd.PersistentFlags().StringVar(&gOpt.dbUser, "user", "root", "db user")
    rootCmd.PersistentFlags().StringVar(&gOpt.dbName, "db-name", "test", "db name to test")
    rootCmd.PersistentFlags().StringVar(&gOpt.dbPassword, "password", "", "password to connecto to db")
    rootCmd.PersistentFlags().IntVar(&gOpt.dbPort, "port", 3306, "db port")
    rootCmd.PersistentFlags().Int64Var(&gOpt.numRows, "num-rows", 1000, "number of rows to generate")

    cmd := &cobra.Command{
            Use:          "test",
            Short:        "test",
            Long:         "test",
            SilenceUsage: true,
            RunE: func(cmd *cobra.Command, args []string) error {
                fmt.Printf("Calling the function. \n")
                var tableComp [][]string
                tableComp = append(tableComp, []string{"point 01", "point 02", "st_distince", "st_distince_sphere", "golang", "query", "st_distince" , "golang", "query"})

                fmt.Printf("The args are <%#v> \n", args)

                for num :=0; num < 10; num++{
                    lat01, lng01 := getPoint(20, 24, 136, 154)
                    fmt.Printf("Point: %f, %f \n", lat01, lng01)
                    lat02, lng02 := getPoint(20, 24, 136, 154)
                    fmt.Printf("Point: %f, %f \n", lat02, lng02)

                    dis01 := EarthDistance(lat01, lng01, lat02, lng02)
                    // fmt.Println(EarthDistance(lat1, lng1, lat2, lng2))

                    dis02, err := earthDistanceFromMySQL(&gOpt, lat01, lng01, lat02, lng02)
                    if err != nil {
                        panic(err)
                    }
                    dis03, err := earthDistanceFromMySQL02(&gOpt, lat01, lng01, lat02, lng02)
                    if err != nil {
                        panic(err)
                    }
                    dis05, err :=  earthDistanceFromMySQLQuery(&gOpt, lat01, lng01, lat02, lng02)
                    if err != nil {
                        panic(err)
                    }
                    // dis04, err := earthDistanceFromTiDB(args, lat01, lng01, lat02, lng02)
                    // if err != nil {
                    //     panic(err)
                    // }

                    fmt.Printf("Distince from calculation: %f \n", dis01)
                    // fmt.Printf("Distince from TiDB calculation: %f \n", dis04)
                    fmt.Printf("Distince from mysql: %f and diff: %f, gosa: %f \n", dis02, dis02 - dis01, 100 * math.Abs(dis02 - dis01)/dis01)
                    fmt.Printf("Distince from mysql: %f and diff: %f, gosa: %f \n", dis03, dis03 - dis01, 100 * math.Abs(dis03 - dis01)/dis01)

                    // fmt.Printf("Distince from mysql: %f and diff: %f, gosa: %f \n", dis02, dis02 - dis04, 100 * math.Abs(dis02 - dis04)/dis04)
                    // fmt.Printf("Distince from mysql: %f and diff: %f, gosa: %f \n", dis03, dis03 - dis04, 100 * math.Abs(dis03 - dis04)/dis04)

                    tableComp = append(tableComp, []string{fmt.Sprintf("(%f, %f)", lat01, lng01), fmt.Sprintf("(%f, %f)", lat02, lng02),
                        fmt.Sprintf("%f", dis02), fmt.Sprintf("%f", dis03), fmt.Sprintf("%f", dis01), fmt.Sprintf("%f", dis05),
                        fmt.Sprintf("%f", 100 * math.Abs(dis02 - dis03)/dis03),
                        fmt.Sprintf("%f", 100 * math.Abs(dis01 - dis03)/dis03),
                        fmt.Sprintf("%f", 100 * math.Abs(dis05 - dis03)/dis03),
                    })
                }

                tui.PrintTable(tableComp, true)
                return nil
            },
    }

    rootCmd.AddCommand(cmd)

    if err := rootCmd.Execute(); err != nil {
        panic(err)
    }

    return

    //args, err := readArgs()
    //if err != nil {
    //    panic(err)
    //}

}

type Args struct {
    dbType     string
    dbHost     string
    dbPort     int
    dbName     string
    dbUser     string
    dbPassword string
    numRows    int64
}

func readArgs() (*Args, error) {
     var args Args
     flag.StringVar(&args.dbType, "db-type", "TiDB", "db type(TiDB or MySQL)")
     flag.StringVar(&args.dbHost, "host", "127.0.0.1", "db host")
     flag.StringVar(&args.dbUser, "user", "root", "db user")
     flag.StringVar(&args.dbName, "db-name", "test", "db name to test")
     flag.StringVar(&args.dbPassword, "password", "", "password to connecto to db")
     flag.IntVar(&args.dbPort, "port", 3306, "db port")
     flag.Int64Var(&args.numRows, "num-rows", 1000, "number of rows to generate")

    // // This declares `numb` and `fork` flags, using a
    // // similar approach to the `word` flag.
    // numbPtr := flag.Int("numb", 42, "an int")
    // boolPtr := flag.Bool("fork", false, "a bool")

    // // It's also possible to declare an option that uses an
    // // existing var declared elsewhere in the program.
    // // Note that we need to pass in a pointer to the flag
    // // declaration function.
    // var svar string
    // flag.StringVar(&svar, "svar", "bar", "a string var")

    // Once all flags are declared, call `flag.Parse()`
    // to execute the command-line parsing.
    flag.Parse()

    return &args, nil
}

func earthDistanceFromMySQL(args *Args, lat1, lng1, lat2, lng2 float64) (float64, error){
   // db, err := sql.Open("mysql", fmt.Sprintf("mysqluser:1234Abcd@tcp(127.0.0.1:3306)/test", args.dbUser, args.dbPassword, args.dbHost, args.dbPort, args.dbName  ) )
   db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", args.dbUser, args.dbPassword, args.dbHost, args.dbPort, args.dbName  ) )

   // if there is an error opening the connection, handle it
   if err != nil {
       return 0, err
   }

   // defer the close till after the main function has finished
   // executing
   defer db.Close()

   // results, err := db.Query(fmt.Sprintf("SELECT ST_Distance_Sphere(Point(%f,%f), Point(%f,%f));", lng1, lat1, lng2, lat2 ) )
   results, err := db.Query(fmt.Sprintf("SELECT 111195*ST_Distance(Point(%f,%f), Point(%f,%f));", lng1, lat1, lng2, lat2 ) )
   if err != nil {
       return 0, err
   }

   for results.Next() {
        var _result float64
        // for each row, scan the result into our tag composite object
        err = results.Scan(&_result)
        if err != nil {
            return 0, err
        }
        return _result, nil
        // and then print out the tag's Name attribute
        //fmt.Printf("Getting: %f \n", _result)
    }
    return 0, nil
}

func earthDistanceFromMySQLQuery(args *Args, lat1, lng1, lat2, lng2 float64) (float64, error){
   // db, err := sql.Open("mysql", fmt.Sprintf("mysqluser:1234Abcd@tcp(127.0.0.1:3306)/test", args.dbUser, args.dbPassword, args.dbHost, args.dbPort, args.dbName  ) )
   db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", args.dbUser, args.dbPassword, args.dbHost, args.dbPort, args.dbName  ) )

   // if there is an error opening the connection, handle it
   if err != nil {
       return 0, err
   }

   // defer the close till after the main function has finished
   // executing
   defer db.Close()

   // results, err := db.Query(fmt.Sprintf("SELECT ST_Distance_Sphere(Point(%f,%f), Point(%f,%f));", lng1, lat1, lng2, lat2 ) )
   results, err := db.Query(fmt.Sprintf("select ROUND(6378.138*2*ASIN(SQRT(POW(SIN((%f*PI()/180-%f*PI()/180)/2),2)+COS(%f*PI()/180)*COS(%f*PI()/180)*POW(SIN((%f*PI()/180-%f*PI()/180)/2),2)))*1000) AS distance", lat1, lat2, lat1, lat2, lng1, lng2 ) )
   if err != nil {
       return 0, err
   }

   for results.Next() {
        var _result float64
        // for each row, scan the result into our tag composite object
        err = results.Scan(&_result)
        if err != nil {
            return 0, err
        }
        return _result, nil
        // and then print out the tag's Name attribute
        //fmt.Printf("Getting: %f \n", _result)
    }
    return 0, nil
}

func earthDistanceFromTiDB(args *Args, lat1, lng1, lat2, lng2 float64) (float64, error){
   // db, err := sql.Open("mysql", fmt.Sprintf("mysqluser:1234Abcd@tcp(127.0.0.1:3306)/test", args.dbUser, args.dbPassword, args.dbHost, args.dbPort, args.dbName  ) )
   db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", args.dbUser, args.dbPassword, args.dbHost, args.dbPort, args.dbName  ) )

   // if there is an error opening the connection, handle it
   if err != nil {
       return 0, err
   }

   // defer the close till after the main function has finished
   // executing
   defer db.Close()

   // results, err := db.Query(fmt.Sprintf("SELECT ST_Distance_Sphere(Point(%f,%f), Point(%f,%f));", lng1, lat1, lng2, lat2 ) )
   results, err := db.Query(fmt.Sprintf("select ROUND(6378.138*2*ASIN(SQRT(POW(SIN((%f*PI()/180-%f*PI()/180)/2),2)+COS(%f*PI()/180)*COS(%f*PI()/180)*POW(SIN((%f*PI()/180-%f*PI()/180)/2),2)))*1000) AS distance", lat1, lat2, lat1, lat2, lng1, lng2 ) )
   if err != nil {
       return 0, err
   }

   for results.Next() {
        var _result float64
        // for each row, scan the result into our tag composite object
        err = results.Scan(&_result)
        if err != nil {
            return 0, err
        }
        return _result, nil
        // and then print out the tag's Name attribute
        //fmt.Printf("Getting: %f \n", _result)
    }
    return 0, nil
}

func earthDistanceFromMySQL02(args *Args, lat1, lng1, lat2, lng2 float64) (float64, error){
   // db, err := sql.Open("mysql", "mysqluser:1234Abcd@tcp(127.0.0.1:3306)/test")
   db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", args.dbUser, args.dbPassword, args.dbHost, args.dbPort, args.dbName  ) )

   // if there is an error opening the connection, handle it
   if err != nil {
       return 0, err
   }

   // defer the close till after the main function has finished
   // executing
   defer db.Close()

   // results, err := db.Query(fmt.Sprintf("SELECT ST_Distance_Sphere(Point(%f,%f), Point(%f,%f));", lng1, lat1, lng2, lat2 ) )
   results, err := db.Query(fmt.Sprintf("SELECT ST_Distance_Sphere(Point(%f,%f), Point(%f,%f));", lng1, lat1, lng2, lat2 ) )
   if err != nil {
       return 0, err
   }

   for results.Next() {
        var _result float64
        // for each row, scan the result into our tag composite object
        err = results.Scan(&_result)
        if err != nil {
            return 0, err
        }
        return _result, nil
        // and then print out the tag's Name attribute
        //fmt.Printf("Getting: %f \n", _result)
    }
    return 0, nil
}


func getPoint(fromLat, toLat, fromLng, toLng float64) (float64, float64){
    s1 := rand.NewSource(time.Now().UnixNano())
    r1 := rand.New(s1)

    lat := float64(int(fromLat) + r1.Intn(int(toLat-fromLat + 1))) + r1.Float64()
    lng := float64(int(fromLng) + r1.Intn(int(toLng-fromLng + 1))) + r1.Float64()

    return lat, lng
}

func EarthDistance(lat1, lng1, lat2, lng2 float64) float64 {
   radius := 6378137
   // radius := 6371000 // 6378137
   // radius := 6370980 // Tested from query
   rad := math.Pi/180.0

   lat1 = lat1 * rad
   lng1 = lng1 * rad
   lat2 = lat2 * rad
   lng2 = lng2 * rad

   theta := lng2 - lng1
   dist := math.Acos(math.Sin(lat1) * math.Sin(lat2) + math.Cos(lat1) * math.Cos(lat2) * math.Cos(theta))

   return dist * float64(radius)
}
