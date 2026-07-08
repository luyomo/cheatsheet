Here is the complete end-to-end breakdown of what happens when a Pod behind an external load balancer using `externalTrafficPolicy: Local` is deleted in GKE.

## Quick Flow Diagram

```mermaid
flowchart TD
    A[User runs kubectl delete pod] --> B[API Server updates etcd]
    B --> C[Pod marked Terminating]
    C --> D[Pod Ready = false]

    C --> E[EndpointSlice updated<br/>Ready = false<br/>Terminating = true]
    E --> F[GKE NEG controller deregisters Pod IP]
    F --> G[kube-proxy updates local health check]
    G --> H{Any local Pod left on this node?}
    H -- No --> I[Health check returns 503]
    I --> J[Load balancer removes node from rotation]
    H -- Yes --> K[Health check stays 200]

    C --> L[Kubelet sees deletion]
    L --> M[preStop hook starts]
    M --> N[Application keeps shutting down gracefully]

    J --> O[No new connections to this Pod]
    K --> O
    O --> P{connectionPersistenceOnUnhealthyBackends}
    P -- NEVER_PERSIST --> P1[Existing connections stop being kept once backend is unhealthy]
    P -- Default --> P2[Existing connections drain for LB timeout]
    P1 --> R
    P2 --> Q[Drain timeout expires]
    Q --> R[Late packets dropped / client may see TCP RST or 502]

    N --> S[preStop finishes]
    S --> T[Kubelet sends SIGTERM]
    T --> U[Container exits]
    U --> V[Pod object removed from etcd]

    classDef red fill:#ffe5e5,stroke:#cc0000,stroke-width:2px,color:#660000;
    class C,D,I,J,P1,R,V red;
```

## At A Glance

- `kubectl delete pod` immediately marks the Pod as `Terminating` and `Ready: false`.
  * [kubernetes Finalizers](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/)
  * [Pod Termination Flow](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination-flow)
- Two things happen in parallel: network removal starts, and the Pod's `preStop` hook starts.
> [!WARNING]
> The node health check can turn unhealthy almost immediately after `kubectl delete pod`, typically somewhere in the `0s-15s` window, long before `preStop` finishes.
- The load balancer stops sending new connections quickly, but existing TCP sessions can drain for the configured timeout.
- After `preStop` completes, the container receives `SIGTERM`, exits, and the Pod is fully removed.

* **`kubectl delete pod`** is executed.
* The API Server updates **`etcd`**, applying a `deletionTimestamp` and setting a countdown based on `terminationGracePeriodSeconds` (default 30s).
* The Pod status instantly shifts to **`Terminating`** and its network state drops to **`Ready: false`**.

### 2. Parallel Actions Triggered Instantly (`T = 0s - 5s`)

#### Track A: The Network Infrastructure

* **EndpointSlice Controller:** Detects the `deletionTimestamp` and modifies the Pod's endpoint inside the `EndpointSlice` object (`Ready: false`, `Terminating: true`).
* **GKE NEG Controller:** Detects the EndpointSlice status change and calls the Google Cloud Engine API to **deregister** the Pod IP from the network endpoint group (NEG).
* **`kube-proxy` & Health Check NodePort:**
* If this was the *only* Pod for the service on that specific worker node, `kube-proxy` updates the node's local health check port (e.g., `31795`) response from `HTTP 200 (localEndpoints: 1)` to **`HTTP 503 (localEndpoints: 0)`**.
* **Timing note:** This health check becoming unhealthy is one of the earliest externally visible changes after `kubectl delete pod`. In practice, it usually flips during this early `0s-15s` period, and it is not blocked by the Pod's `preStop` sleep.
  ```sh
  ...
  spec:
    type: LoadBalancer
    externalTrafficPolicy: Local
    healthCheckNodePort: 31795
  ...
  ```
  * [nlb load balancer](https://docs.cloud.google.com/kubernetes-engine/docs/concepts/service-load-balancer)
  * [EndpointSlice Conditions](https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/#conditions)
* The Cloud Load Balancer drops the entire node from rotation for this service.



#### Track B: The Local Worker Node

* **Kubelet:** Monitors `etcd`, notices the deletion request, and immediately runs your configured **`preStop` hook** (e.g., a 30-second sleep).
* **Crucial Detail:** The `preStop` hook runs *in parallel* with the network updates above. It does not delay them.

### 3. Load Balancer Connection Draining (`T = 5s - 20s`)

* **New Connections:** The Cloud Load Balancer completely stops sending brand-new client connections to the terminating Pod.
* **Branch: `connectionPersistenceOnUnhealthyBackends`:**
* If set to **`NEVER_PERSIST`**, existing connections are **not** kept on an unhealthy backend. In this Pod deletion flow, that means the old connection can stop being honored once the backend is considered unhealthy by the load balancer, instead of waiting for the connection draining timeout.
* If left at the **default behavior** (`DEFAULT_FOR_PROTOCOL`), eligible existing TCP connections can continue during the configured **Connection Draining Timeout**. In this example, that timeout is 15 seconds.
* **The Hard Cutoff For Default Behavior:** Once the draining timeout expires (at 15s), the load balancer evicts the Pod from its connection tracking table. Any late packets sent on those old connections are dropped, and the client receives a connection error (`TCP RST` or `502 Bad Gateway`). They are *not* automatically rerouted to other nodes.
  * [Connection draining](https://docs.cloud.google.com/load-balancing/docs/enabling-connection-draining)
  * [ComputeBackendService connectionTrackingPolicy](https://docs.cloud.google.com/config-connector/docs/reference/resource-docs/compute/computebackendservice)

### 4. Application Teardown (`T = 30s+`)

* Your 30-second `preStop` hook finishes sleeping.
* The Kubelet finally sends a polite **`SIGTERM`** signal to PID 1 inside the container, giving the application a brief window to save state and flush internal queues.
* **Final Cleanup:** Once the container processes exit completely, the Kubelet notifies the API Server, which permanently removes the Pod object from `etcd`. The Pod vanishes from `kubectl get pods`.
