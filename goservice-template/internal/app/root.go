package app

import (
    "fmt"

    "github.com/luyomo/cheatsheet/goservice-template/internal/app/configs"
)

func Run(gOpt configs.Options) error {
    fmt.Println("Executing the application")

    fmt.Printf("The options are : %#v \n", gOpt)

    config, err := configs.ReadConfigFile(gOpt.ConfigFile)
    if err != nil {
        return err 
    }
    fmt.Printf("The configs are : %#v \n", config)

    return nil
}
