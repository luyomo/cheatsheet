package main


import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"fmt"
)


func main() {
dsn := "root:1234Abcd@tcp(10.0.1.4:4000)/messagedb?parseTime=true"

db, err := sql.Open("mysql", dsn)
if err != nil {
	fmt.Println("failed to open MySQL:", err)
	return
}
defer db.Close()

db.SetMaxOpenConns(10)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(0)

if err := db.Ping(); err != nil {
	fmt.Println("failed to connect:", err)
	return
}

rows, err := db.Query("SELECT Host, user FROM mysql.user LIMIT 10")
if err != nil {
	fmt.Println("query error:", err)
	return
}
defer rows.Close()

for rows.Next() {
	var id string
	var name string
	if err := rows.Scan(&id, &name); err != nil {
		fmt.Println("scan error:", err)
		return
	}
	fmt.Printf("id=%s name=%s\n", id, name)
}
if err := rows.Err(); err != nil {
	fmt.Println("rows error:", err)
	return
}
}
