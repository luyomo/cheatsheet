import concurrent.futures
import os
import random
import sys
import time
import traceback
from dataclasses import dataclass

from tidbcloudlake_driver import BlockingLakeClient

# --- BENCHMARK CONFIGURATION ---
CONCURRENT_THREADS = 32        # Number of parallel worker threads simulating clients
TEST_DURATION_SECONDS = 30    # How long to run the load test
CONNECTION_URL = os.environ["LAKE_CONNECTION_URL"]

# --- SAMPLE DATA FOR POINT GET ---
# Replace these samples with valid combinations present in your audience_segment_data table!
SAMPLE_LOOKUPS = [
    {"ctwid": "usr_7426047", "appid": "app_018", "tag_key": "user_interest"},
    {"ctwid": "usr_5513956", "appid": "app_013", "tag_key": "user_interest"},
    # Add more real entries here to avoid hitting the exact same row repeatedly if testing cache bypass
]

@dataclass
class WorkerStats:
    latencies: list[float]
    total_queries: int = 0
    total_errors: int = 0
    total_misses: int = 0


def worker_thread_loop(url, lookups, stop_time):
    """
    Each thread creates its own client instance locally inside its own context.
    This guarantees zero thread cross-contamination.
    """
    local_stats = WorkerStats(latencies=[])
    client = None
    cursor = None
    query_template = """
        SELECT `tag_value`, `updated`
        FROM `audience_segment_data`
        WHERE `ctwid` = ? AND `appid` = ? AND `tag_key` = ?
        LIMIT 1;
    """

    try:
        # Initialize the client inside the thread
        client = BlockingLakeClient(url)
        cursor = client.cursor()
    except Exception:
        print(f"\n❌ [CRITICAL] Thread failed to initialize network driver context:")
        traceback.print_exc(file=sys.stdout)
        raise

    try:
        while time.time() < stop_time:
            target = random.choice(lookups)
            params = (target["ctwid"], target["appid"], target["tag_key"])

            start_query = time.perf_counter()
            try:
                cursor.execute(query_template, params)
                row = cursor.fetchone()
                latency = (time.perf_counter() - start_query) * 1000
                local_stats.latencies.append(latency)
                local_stats.total_queries += 1

                if row is None:
                    local_stats.total_misses += 1
            except Exception as exc:
                local_stats.total_errors += 1
                print("\n❌ [CRITICAL] Query execution failed during worker thread")
                print(f"Query params: ctwid={params[0]}, appid={params[1]}, tag_key={params[2]}")
                print(f"Exception type: {type(exc).__name__}")
                print(f"Exception detail: {exc}")
                traceback.print_exc(file=sys.stdout)
                break
    finally:
        print("Process complete")
#        if cursor is not None:
#            cursor.close()
#        if client is not None:
#            client.close()

    return local_stats

if __name__ == "__main__":
    print("🔌 Initializing Cloud Lake client connections...")

    # 2. FORCE a real network handshake on the main thread immediately
    print("📡 Testing live network connection handshake to TiDB Cloud Lake...")
    test_client = BlockingLakeClient(CONNECTION_URL)
    try:
        test_cursor = test_client.cursor()
        print("✅ Core network handshake successful!")
        test_cursor.execute("SELECT 1 as col01;")
        test_v = test_cursor.fetchone()
        print(test_v['col01'])
        test_cursor.close()
        print("✅ Simple query execution verified!")
    except Exception:
        print("\n❌ [CRITICAL] Connection Handshake Failed on Main Thread!")
        print("=" * 60)
        traceback.print_exc(file=sys.stdout)
        print("=" * 60)
        sys.exit(1)
    finally:
        print("Complete")
#        test_client.close()

    global_stats = {
        "latencies": [],
        "total_queries": 0,
        "total_errors": 0,
        "total_misses": 0,
    }

    print(f"🏁 Starting point-get benchmark using {CONCURRENT_THREADS} threads for {TEST_DURATION_SECONDS}s...")
    start_benchmark = time.time()
    end_boundary = start_benchmark + TEST_DURATION_SECONDS

    # Execute concurrent worker pools
    with concurrent.futures.ThreadPoolExecutor(max_workers=CONCURRENT_THREADS) as executor:
        futures = [
            executor.submit(worker_thread_loop, CONNECTION_URL, SAMPLE_LOOKUPS, end_boundary)
            for _ in range(CONCURRENT_THREADS)
        ]
        for future in concurrent.futures.as_completed(futures):
            worker_stats = future.result()
            global_stats["latencies"].extend(worker_stats.latencies)
            global_stats["total_queries"] += worker_stats.total_queries
            global_stats["total_errors"] += worker_stats.total_errors
            global_stats["total_misses"] += worker_stats.total_misses

    actual_duration = time.time() - start_benchmark
    all_latencies = sorted(global_stats["latencies"])

    # --- REPORTING TELEMETRY ---
    print("\n" + "="*40)
    print("📈 BENCHMARK REPORT")
    print("="*40)
    
    if all_latencies:
        qps = global_stats["total_queries"] / actual_duration
        avg_latency = sum(all_latencies) / len(all_latencies)
        p95_latency = all_latencies[int(len(all_latencies) * 0.95)]
        p99_latency = all_latencies[int(len(all_latencies) * 0.99)]
        
        print(f"Total Successful Queries : {global_stats['total_queries']}")
        print(f"Total Failed Queries     : {global_stats['total_errors']}")
        print(f"Total Empty Results      : {global_stats['total_misses']}")
        print(f"Actual Duration          : {actual_duration:.2f} seconds")
        print(f"Throughput (QPS)         : {qps:.2f} queries/sec")
        print("-" * 40)
        print(f"Average Latency          : {avg_latency:.2f} ms")
        print(f"95th Percentile (P95)    : {p95_latency:.2f} ms")
        print(f"99th Percentile (P99)    : {p99_latency:.2f} ms")
    else:
        print("❌ No successful queries completed. Check connection or error parameters.")
    print("="*40)
