package main

import (
    "fmt"
    "context"

    "github.com/tikv/client-go/v2/txnkv"
)

func main(){
    err := insertData()
    if err != nil {
        panic(err)
    }

    err = selectData()
    if err != nil {
        panic(err)
    }

    err = selectSnapshotData()
    if err != nil {
        panic(err)
    }

    err = deleteData()
    if err != nil {
        panic(err)
    }

    err = selectData()
    if err != nil {
        panic(err)
    }

    return
}

func insertData() error {
    client, err := txnkv.NewClient([]string{"10.128.0.21:2379"})
    if err != nil {
        return err
    }

    //txn, err := client.Begin(opts...)
    txn, err := client.Begin()
    if err != nil {
        return err
    }

    if err := txn.Set([]byte("foo"), []byte("bar")); err != nil {
        return err
    }

    if err := txn.Commit(context.TODO()); err != nil {
        return err
    }

    return nil
}

func deleteData() error {
    client, err := txnkv.NewClient([]string{"10.128.0.21:2379"})
    if err != nil {
        return err
    }

    //txn, err := client.Begin(opts...)
    txn, err := client.Begin()
    if err != nil {
        return err
    }

    if err := txn.Delete([]byte("foo")); err != nil {
        return err
    }

    if err := txn.Commit(context.TODO()); err != nil {
        return err
    }

    return nil
}

func selectData() error {
    client, err := txnkv.NewClient([]string{"10.128.0.21:2379"})
    if err != nil {
        return err
    }

    txn, err := client.Begin()
    if err != nil {
        return err
    }

    v, err := txn.Get(context.TODO(), []byte("foo"))
    if err != nil {
        return err
    }

    fmt.Printf("The tikv value is: %s \n", string(v))

    return nil
}

func selectSnapshotData() error {
    client, err := txnkv.NewClient([]string{"10.128.0.21:2379"})
    if err != nil {
        return err
    }

    ts, err := client.CurrentTimestamp("global")
    if err != nil {
        return err
    }

    snapshot := client.GetSnapshot(ts)
    v, err := snapshot.Get(context.TODO(), []byte("foo"))
    if err != nil {
        return err
    }
    fmt.Printf("Fetch data from snapshot: %s \n", string(v))

    return nil
}
