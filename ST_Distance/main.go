package main

import (
   "fmt"
   "math"
   "math/rand"
   "time"

   "database/sql"
   _ "github.com/go-sql-driver/mysql"

)

// https://www.gsi.go.jp/KOKUJYOHO/center.htm
// 136 |<- ->| 154
//  20 |<- ->|  24


// SELECT xxx,st_distance($geo,geo)*111195 as distance FROM `$schema` WHERE  geo is not null and st_distance($geo,geo)*111195 <= :distance order by distance
// create table test.gis_table(id bigint primary key, name varchar(32) not null, position point)

func main() {
   // lat1 := 29.490295
   // lng1 := 106.486654

   // lat2 := 29.615467
   // lng2 := 126.581515

   lat01, lng01 := getPoint(20, 24, 136, 154)
   fmt.Printf("Point: %f, %f \n", lat01, lng01)
   lat02, lng02 := getPoint(20, 24, 136, 154)
   fmt.Printf("Point: %f, %f \n", lat02, lng02)

   dis01 := EarthDistance(lat01, lng01, lat02, lng02)
   // fmt.Println(EarthDistance(lat1, lng1, lat2, lng2))

   dis02, err := earthDistanceFromMySQL(lat01, lng01, lat02, lng02)
   if err != nil {
       panic(err)
   }
   dis03, err := earthDistanceFromMySQL02(lat01, lng01, lat02, lng02)
   if err != nil {
       panic(err)
   }
   dis04, err := earthDistanceFromTiDB(lat01, lng01, lat02, lng02)
   if err != nil {
       panic(err)
   }

   fmt.Printf("Distince from calculation: %f \n", dis01)
   fmt.Printf("Distince from TiDB calculation: %f \n", dis04)
   fmt.Printf("Distince from mysql: %f and diff: %f, gosa: %f \n", dis02, dis02 - dis01, 100 * math.Abs(dis02 - dis01)/dis01)
   fmt.Printf("Distince from mysql: %f and diff: %f, gosa: %f \n", dis03, dis03 - dis01, 100 * math.Abs(dis03 - dis01)/dis01)

   fmt.Printf("Distince from mysql: %f and diff: %f, gosa: %f \n", dis02, dis02 - dis04, 100 * math.Abs(dis02 - dis04)/dis04)
   fmt.Printf("Distince from mysql: %f and diff: %f, gosa: %f \n", dis03, dis03 - dis04, 100 * math.Abs(dis03 - dis04)/dis04)
}

func earthDistanceFromMySQL(lat1, lng1, lat2, lng2 float64) (float64, error){
   db, err := sql.Open("mysql", "mysqluser:1234Abcd@tcp(127.0.0.1:3306)/test")

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
    
    // ROUND(6378.138*2*ASIN(SQRT(POW(SIN(({$lat}*PI()/180-lat*PI()/180)/2),2)+COS({$lat}*PI()/180)*COS(lat*PI()/180)*POW(SIN(({$lng}*PI()/180-lng*PI()/180)/2),2)))*1000) AS distance 
}

func earthDistanceFromTiDB(lat1, lng1, lat2, lng2 float64) (float64, error){
   db, err := sql.Open("mysql", "mysqluser:1234Abcd@tcp(127.0.0.1:3306)/test")

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
    
    // ROUND(6378.138*2*ASIN(SQRT(POW(SIN(({$lat}*PI()/180-lat*PI()/180)/2),2)+COS({$lat}*PI()/180)*COS(lat*PI()/180)*POW(SIN(({$lng}*PI()/180-lng*PI()/180)/2),2)))*1000) AS distance 
}

func earthDistanceFromMySQL02(lat1, lng1, lat2, lng2 float64) (float64, error){
   db, err := sql.Open("mysql", "mysqluser:1234Abcd@tcp(127.0.0.1:3306)/test")

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
