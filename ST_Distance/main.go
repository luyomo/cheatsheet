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

func main() {
   // lat1 := 29.490295
   // lng1 := 106.486654

   // lat2 := 29.615467
   // lng2 := 126.581515

   lat01, lng01 := getPoint(20, 24, 136, 154)
   fmt.Printf("Point: %f, %f \n", lat01, lng01)
   lat02, lng02 := getPoint(20, 24, 136, 154)
   fmt.Printf("Point: %f, %f \n", lat02, lng02)

   fmt.Println(EarthDistance(lat01, lng01, lat02, lng02))
   // fmt.Println(EarthDistance(lat1, lng1, lat2, lng2))
}

func earthDistanceFromMySQL() {
   db, err := sql.Open("mysql", "username:password@tcp(127.0.0.1:3306)/test")

   // if there is an error opening the connection, handle it
   if err != nil {
       panic(err.Error())
   }

   // defer the close till after the main function has finished
   // executing
   defer db.Close()
}

func getPoint(fromLat, toLat, fromLng, toLng float64) (float64, float64){
    s1 := rand.NewSource(time.Now().UnixNano())
    r1 := rand.New(s1)

    lat := float64(int(fromLat) + r1.Intn(int(toLat-fromLat + 1))) + r1.Float64()
    lng := float64(int(fromLng) + r1.Intn(int(toLng-fromLng + 1))) + r1.Float64()

    return lat, lng
}

func EarthDistance(lat1, lng1, lat2, lng2 float64) float64 {
   // radius := 6371000 // 6378137
   radius := 6370980 // Tested from query
   rad := math.Pi/180.0

   lat1 = lat1 * rad
   lng1 = lng1 * rad
   lat2 = lat2 * rad
   lng2 = lng2 * rad

   theta := lng2 - lng1
   dist := math.Acos(math.Sin(lat1) * math.Sin(lat2) + math.Cos(lat1) * math.Cos(lat2) * math.Cos(theta))

   return dist * float64(radius)
}
