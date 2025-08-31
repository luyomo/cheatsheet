package main

import (
    "database/sql"
    "fmt"
    "net/http"

    "github.com/gin-gonic/gin"
    _ "github.com/go-sql-driver/mysql"
)

// Request payload structure
type DBRequest struct {
    Host     string `json:"host" binding:"required"`
    Port     int    `json:"port" binding:"required"`
    User     string `json:"user" binding:"required"`
    Password string `json:"password" binding:"required"`
    DBName   string `json:"dbname" binding:"required"`
}

// Response payload
type DBResponse struct {
    Success bool   `json:"success"`
    Message string `json:"message"`
}

func main() {
    r := gin.Default()

    r.POST("/check", func(c *gin.Context) {
        var req DBRequest
        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(http.StatusBadRequest, DBResponse{
                Success: false,
                Message: fmt.Sprintf("Invalid request: %v", err),
            })
            return
        }

        // Build DSN (Data Source Name)
        dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", req.User, req.Password, req.Host, req.Port, req.DBName)

        // Try connecting
        db, err := sql.Open("mysql", dsn)
        if err != nil {
            c.JSON(http.StatusInternalServerError, DBResponse{
                Success: false,
                Message: fmt.Sprintf("Connection error: %v", err),
            })
            return
        }
        defer db.Close()

        // Ping DB
        if err := db.Ping(); err != nil {
            c.JSON(http.StatusInternalServerError, DBResponse{
                Success: false,
                Message: fmt.Sprintf("Ping failed: %v", err),
            })
            return
        }

        c.JSON(http.StatusOK, DBResponse{
            Success: true,
            Message: "Database connection successful!",
        })
    })

    r.Run("0.0.0.0:8080") // start server
}

