package main

import (
    "fmt"
    "time"
    "os"
    "context"

    "github.com/prometheus/client_golang/api"
    "github.com/prometheus/common/model"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
    "github.com/luyomo/tidbcloud-sdk-go-v1/pkg/tidbcloud"
)

func main() {
    client, err := api.NewClient(api.Config{
		Address: "http://localhost:9090",
	})
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}

    ticker := time.NewTicker(time.Minute)
    for {
       select {
           case t := <-ticker.C:
               fmt.Println("Tick at", t)

	           v1api := v1.NewAPI(client)
	           ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	           defer cancel()
	           r := v1.Range{
	           	   Start: time.Now().Add(-time.Minute * 5),
	           	   End:   time.Now(),
	           	   Step:  time.Minute,
	           }
	           result, warnings, err := v1api.QueryRange(ctx, "rate(tidbcloud_node_cpu_seconds_total{cluster_name=\"scalingtest\", component=\"tidb\"}[5m])", r, v1.WithTimeout(10*time.Second))
	           if err != nil {
	               fmt.Printf("Error querying Prometheus: %v\n", err)
	               os.Exit(1)
	           }
	           if len(warnings) > 0 {
	           	fmt.Printf("Warnings: %v\n", warnings)
	           }

               idx := 0
               for _, val := range result.(model.Matrix)[0].Values {
                   fmt.Printf("Result: %s:%s\n", val.Timestamp, val.Value)
                   if val.Value < 1.6 {
                       continue
                   }
                   idx = idx + 1
               }
               if idx == len(result.(model.Matrix)[0].Values) {
                   fmt.Printf("Starting to scale out the nodes \n")
                   if err := scalingTiDBOut(3, 1); err != nil {
                       panic(err)
                   }
               }else {
                   fmt.Printf("No need to scale out the nodes \n")
               }
       }
    }
}

func scalingTiDBOut(maxNodes, scales int32) error {
    // Fetch TiDB Info using API
    numTiDBNodes, err := getTiDBNumNodes()
    if err != nil {
        return err
    }
    fmt.Printf("The current nodes are : %d \n", numTiDBNodes)

    // If the current number nodes exceed the maxNodes, finish
    if int32(numTiDBNodes) < maxNodes {
        fmt.Printf("Starting to add one new node \n")
        
        if err := scaleTidb(int32(numTiDBNodes)+scales); err != nil {
            return err
        }
    }
    
    // If the current number nodes is less then maxNodes, add min(scales, maxNodes - currentNodes)

    return nil
}

func getTiDBNumNodes() (int, error) {
    resp, err := getCluster()
    if err != nil {
        return 0, err
    }

    fmt.Printf("The nodes: <%#v> \n", len(resp.JSON200.Status.NodeMap.Tidb) )

    return len(resp.JSON200.Status.NodeMap.Tidb), nil
}

func getCluster() (*tidbcloud.GetClusterResponse, error) {

    projectID := os.Getenv("TIDBCLOUD_PROJECT_ID")
    if projectID == "" {
        panic("No project id is specified")
    }

    clusterName := os.Getenv("TIDBCLOUD_CLUSTER_NAME")
    if clusterName == "" {
        panic("No project id is specified")
    }

    client, err := tidbcloud.NewDigestClientWithResponses()
    if err != nil {
        panic(err)
    }

    response, err := client.ListClustersOfProjectWithResponse(context.Background(), projectID,  &tidbcloud.ListClustersOfProjectParams{})
    if err != nil {
        panic(err)
    }

    for _, item := range response.JSON200.Items {
        if *item.Name == clusterName && (*item.Status.ClusterStatus).(string) == "AVAILABLE" {
            getClusterRes, err := client.GetClusterWithResponse(context.Background(), projectID, item.Id )
            if err != nil {
                panic(err)
            }
            fmt.Printf("Response: <%#v>", len(getClusterRes.JSON200.Status.NodeMap.Tidb) )
            return getClusterRes, nil
        }
    }

    return nil, nil
}

func scaleTidb(numNodes int32) error {

    projectID := os.Getenv("TIDBCLOUD_PROJECT_ID")
    if projectID == "" {
        panic("No project id is specified")
    }

    clusterName := os.Getenv("TIDBCLOUD_CLUSTER_NAME")
    if clusterName == "" {
        panic("No project id is specified")
    }

    client, err := tidbcloud.NewDigestClientWithResponses()
    if err != nil {
        panic(err)
    }

    response, err := client.ListClustersOfProjectWithResponse(context.Background(), projectID,  &tidbcloud.ListClustersOfProjectParams{})
    if err != nil {
        panic(err)
    }

    for _, item := range response.JSON200.Items {
        if *item.Name == clusterName && (*item.Status.ClusterStatus).(string) == "AVAILABLE" {
            fmt.Printf("The cluster id: %s", item.Id)

            updateClusterJSONRequestBody := tidbcloud.UpdateClusterJSONRequestBody{}

            updateClusterJSONRequestBody.Config.Components = &struct{
                Tidb *struct {
                    NodeQuantity *int32 `json:"node_quantity,omitempty"`
                    NodeSize *string `json:"node_size,omitempty"`
                } `json:"tidb,omitempty"`
                Tiflash *struct {
                    NodeQuantity *int32 `json:"node_quantity,omitempty"`
                    NodeSize *string `json:"node_size,omitempty"`
                    StorageSizeGib *int32 `json:"storage_size_gib,omitempty"`
                } `json:"tiflash,omitempty"`
                Tikv *struct {
                    NodeQuantity *int32 `json:"node_quantity,omitempty"`
                    NodeSize *string `json:"node_size,omitempty"`
                    StorageSizeGib *int32 `json:"storage_size_gib,omitempty"`
                } `json:"tikv,omitempty"`
            }{
                &struct {
                    NodeQuantity *int32 `json:"node_quantity,omitempty"`
                    NodeSize *string `json:"node_size,omitempty"`
                }{ &numNodes, nil } ,
                nil,
                nil,
            }

            response, err := client.UpdateClusterWithResponse(context.Background(), projectID, item.Id, updateClusterJSONRequestBody)
            if err != nil {
                panic(err)
            }

            statusCode := response.StatusCode()
            fmt.Printf("Sgtatus code: %s \n", statusCode)



            // getClusterRes, err := client.GetClusterWithResponse(context.Background(), projectID, item.Id )
            // if err != nil {
            //     panic(err)
            // }
            // fmt.Printf("Response: <%#v>", len(getClusterRes.JSON200.Status.NodeMap.Tidb) )
            // return getClusterRes, nil
        }
    }

    return nil
}
